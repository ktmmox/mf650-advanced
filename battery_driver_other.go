//go:build !linux

package main
import "errors"
func batteryDriverRead()([]byte,error){return nil,errors.New("请连接 MF650 设备读取电池")}
func batteryDriverSetPaused(bool)error{return errors.New("仅支持设备本机控制")}
