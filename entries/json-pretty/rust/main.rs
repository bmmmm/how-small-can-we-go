use std::env;
use std::fs;
use std::io::{self, Write};
use std::process;

/// A parsed JSON value. Object members keep insertion order (after
/// last-wins de-duplication); sorting for output happens at print time.
/// Numbers keep their original source text — the spec requires byte-exact
/// preservation, which rules out parsing into f64.
enum Value {
    Null,
    Bool(bool),
    Number(String),
    Str(String),
    Array(Vec<Value>),
    Object(Vec<(String, Value)>),
}

struct Parser {
    chars: Vec<char>,
    pos: usize,
}

impl Parser {
    fn peek(&self) -> Option<char> {
        self.chars.get(self.pos).copied()
    }

    fn skip_ws(&mut self) {
        while matches!(self.peek(), Some(' ' | '\t' | '\n' | '\r')) {
            self.pos += 1;
        }
    }

    fn parse_document(&mut self) -> Result<Value, ()> {
        self.skip_ws();
        let v = self.parse_value()?;
        self.skip_ws();
        if self.pos != self.chars.len() {
            return Err(()); // trailing content: a second value, a stray token
        }
        Ok(v)
    }

    fn parse_value(&mut self) -> Result<Value, ()> {
        self.skip_ws();
        match self.peek() {
            Some('{') => self.parse_object(),
            Some('[') => self.parse_array(),
            Some('"') => Ok(Value::Str(self.parse_string()?)),
            Some('t') => self.parse_keyword("true", Value::Bool(true)),
            Some('f') => self.parse_keyword("false", Value::Bool(false)),
            Some('n') => self.parse_keyword("null", Value::Null),
            Some(c) if c == '-' || c.is_ascii_digit() => Ok(Value::Number(self.parse_number()?)),
            _ => Err(()),
        }
    }

    fn parse_keyword(&mut self, word: &str, value: Value) -> Result<Value, ()> {
        for expect in word.chars() {
            if self.peek() != Some(expect) {
                return Err(());
            }
            self.pos += 1;
        }
        Ok(value)
    }

    // Grammar per RFC 8259: -? (0 | [1-9][0-9]*) (.[0-9]+)? ([eE][+-]?[0-9]+)?
    // The matched source text is returned verbatim for byte-exact output.
    fn parse_number(&mut self) -> Result<String, ()> {
        let start = self.pos;
        if self.peek() == Some('-') {
            self.pos += 1;
        }
        match self.peek() {
            Some('0') => self.pos += 1,
            Some(c) if c.is_ascii_digit() => self.consume_digits(),
            _ => return Err(()),
        }
        if self.peek() == Some('.') {
            self.pos += 1;
            if !matches!(self.peek(), Some(c) if c.is_ascii_digit()) {
                return Err(());
            }
            self.consume_digits();
        }
        if matches!(self.peek(), Some('e' | 'E')) {
            self.pos += 1;
            if matches!(self.peek(), Some('+' | '-')) {
                self.pos += 1;
            }
            if !matches!(self.peek(), Some(c) if c.is_ascii_digit()) {
                return Err(());
            }
            self.consume_digits();
        }
        Ok(self.chars[start..self.pos].iter().collect())
    }

    fn consume_digits(&mut self) {
        while matches!(self.peek(), Some(c) if c.is_ascii_digit()) {
            self.pos += 1;
        }
    }

    fn parse_string(&mut self) -> Result<String, ()> {
        self.pos += 1; // opening quote
        let mut s = String::new();
        loop {
            let c = self.peek().ok_or(())?;
            self.pos += 1;
            match c {
                '"' => return Ok(s),
                '\\' => s.push(self.parse_escape()?),
                c if (c as u32) < 0x20 => return Err(()), // unescaped control char
                c => s.push(c),
            }
        }
    }

    // Handles one escape sequence after the backslash, including the
    // \uXXXX \uXXXX surrogate-pair case for astral code points.
    fn parse_escape(&mut self) -> Result<char, ()> {
        let e = self.peek().ok_or(())?;
        self.pos += 1;
        match e {
            '"' => Ok('"'),
            '\\' => Ok('\\'),
            '/' => Ok('/'),
            'b' => Ok('\u{0008}'),
            'f' => Ok('\u{000C}'),
            'n' => Ok('\n'),
            'r' => Ok('\r'),
            't' => Ok('\t'),
            'u' => {
                let cp = self.parse_hex4()?;
                if (0xD800..=0xDBFF).contains(&cp) {
                    if self.peek() != Some('\\') {
                        return Err(());
                    }
                    self.pos += 1;
                    if self.peek() != Some('u') {
                        return Err(());
                    }
                    self.pos += 1;
                    let low = self.parse_hex4()?;
                    if !(0xDC00..=0xDFFF).contains(&low) {
                        return Err(());
                    }
                    let combined = 0x10000 + (cp - 0xD800) * 0x400 + (low - 0xDC00);
                    char::from_u32(combined).ok_or(())
                } else if (0xDC00..=0xDFFF).contains(&cp) {
                    Err(()) // lone low surrogate
                } else {
                    char::from_u32(cp).ok_or(())
                }
            }
            _ => Err(()),
        }
    }

    fn parse_hex4(&mut self) -> Result<u32, ()> {
        let mut v: u32 = 0;
        for _ in 0..4 {
            let c = self.peek().ok_or(())?;
            let d = c.to_digit(16).ok_or(())?;
            v = v * 16 + d;
            self.pos += 1;
        }
        Ok(v)
    }

    fn parse_object(&mut self) -> Result<Value, ()> {
        self.pos += 1; // '{'
        let mut members: Vec<(String, Value)> = Vec::new();
        self.skip_ws();
        if self.peek() == Some('}') {
            self.pos += 1;
            return Ok(Value::Object(members));
        }
        loop {
            self.skip_ws();
            if self.peek() != Some('"') {
                return Err(());
            }
            let key = self.parse_string()?;
            self.skip_ws();
            if self.peek() != Some(':') {
                return Err(());
            }
            self.pos += 1;
            let value = self.parse_value()?;
            // Last occurrence wins: drop any earlier member with this key.
            if let Some(i) = members.iter().position(|(k, _)| *k == key) {
                members.remove(i);
            }
            members.push((key, value));
            self.skip_ws();
            match self.peek() {
                Some(',') => self.pos += 1,
                Some('}') => {
                    self.pos += 1;
                    return Ok(Value::Object(members));
                }
                _ => return Err(()),
            }
        }
    }

    fn parse_array(&mut self) -> Result<Value, ()> {
        self.pos += 1; // '['
        let mut elems = Vec::new();
        self.skip_ws();
        if self.peek() == Some(']') {
            self.pos += 1;
            return Ok(Value::Array(elems));
        }
        loop {
            elems.push(self.parse_value()?);
            self.skip_ws();
            match self.peek() {
                Some(',') => self.pos += 1,
                Some(']') => {
                    self.pos += 1;
                    return Ok(Value::Array(elems));
                }
                _ => return Err(()),
            }
        }
    }
}

fn parse_document(text: &str) -> Result<Value, ()> {
    let mut p = Parser {
        chars: text.chars().collect(),
        pos: 0,
    };
    p.parse_document()
}

/// Escapes a decoded string per the task's minimal-escaping rule: the
/// standard JSON control escapes plus U+2028/U+2029 (JS line terminators),
/// nothing else — no HTML escaping of `/ < > &` or similar.
fn write_escaped_string(s: &str, out: &mut String) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\u{0008}' => out.push_str("\\b"),
            '\u{000C}' => out.push_str("\\f"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{2028}' => out.push_str("\\u2028"),
            '\u{2029}' => out.push_str("\\u2029"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
}

fn push_indent(out: &mut String, level: usize) {
    for _ in 0..level {
        out.push_str("  ");
    }
}

fn print_value(v: &Value, level: usize, out: &mut String) {
    match v {
        Value::Null => out.push_str("null"),
        Value::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
        Value::Number(tok) => out.push_str(tok),
        Value::Str(s) => write_escaped_string(s, out),
        Value::Array(elems) => {
            if elems.is_empty() {
                out.push_str("[]");
                return;
            }
            out.push_str("[\n");
            for (i, e) in elems.iter().enumerate() {
                push_indent(out, level + 1);
                print_value(e, level + 1, out);
                if i + 1 < elems.len() {
                    out.push(',');
                }
                out.push('\n');
            }
            push_indent(out, level);
            out.push(']');
        }
        Value::Object(members) => {
            if members.is_empty() {
                out.push_str("{}");
                return;
            }
            // Sort by byte-wise key order (Rust's str Ord already compares
            // by UTF-8 byte value, matching the spec's memcmp rule).
            let mut order: Vec<usize> = (0..members.len()).collect();
            order.sort_by(|&i, &j| members[i].0.cmp(&members[j].0));
            out.push_str("{\n");
            for (pos, &i) in order.iter().enumerate() {
                let (key, val) = &members[i];
                push_indent(out, level + 1);
                write_escaped_string(key, out);
                out.push_str(": ");
                print_value(val, level + 1, out);
                if pos + 1 < order.len() {
                    out.push(',');
                }
                out.push('\n');
            }
            push_indent(out, level);
            out.push('}');
        }
    }
}

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() != 2 {
        process::exit(1);
    }
    let bytes = match fs::read(&args[1]) {
        Ok(b) => b,
        Err(_) => process::exit(1),
    };
    let text = match std::str::from_utf8(&bytes) {
        Ok(t) => t,
        Err(_) => process::exit(1),
    };
    let value = match parse_document(text) {
        Ok(v) => v,
        Err(_) => process::exit(1),
    };

    let mut out = String::new();
    print_value(&value, 0, &mut out);
    out.push('\n');

    let stdout = io::stdout();
    let mut lock = stdout.lock();
    if lock.write_all(out.as_bytes()).is_err() {
        process::exit(1);
    }
}
