//go:build !windows

package main

import "gioui.org/unit"

// initialWindowSize 为尚未接入平台工作区查询的系统提供保守默认值。
// 后续为 macOS / Linux 增加对应的原生工作区与 DPI 读取实现。
func initialWindowSize() (unit.Dp, unit.Dp) {
	return fitInitialWindowSize(1320, 805)
}
