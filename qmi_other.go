//go:build !linux
package main
import "errors"
func nasQuery(id uint16,payload []byte)(map[byte][]byte,error){return nil,errors.New("当前系统没有设备 QMI 接口")}
