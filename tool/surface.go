// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/bmmmm/how-small-can-we-go/tool/internal/arena"
)

func cmdSurface(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("surface: exactly one directory expected")
	}
	m, err := arena.MeasureDir(args[0])
	if err != nil {
		return err
	}
	fmt.Println(m.Surface)
	return nil
}
