//go:build linux

package main

import("os";"syscall";"fmt")
// These command numbers and the 32-byte current layout are verified against the
// installed MF650 factory battery service. H1 pauses charging; H2 permits it.
func batteryDriverRead()([]byte,error){f,e:=os.Open("/dev/IP5332");if e!=nil{return nil,fmt.Errorf("无法读取 IP5332 电源芯片：%w",e)};defer f.Close();b:=make([]byte,32);n,e:=f.Read(b);if e!=nil{return nil,e};return b[:n],nil}
func batteryDriverSetPaused(paused bool)error{f,e:=os.Open("/dev/IP5332");if e!=nil{return e};defer f.Close();request:=uintptr(0x4802);if paused{request=0x4801};_,_,errno:=syscall.Syscall(syscall.SYS_IOCTL,f.Fd(),request,0);if errno!=0{return errno};return nil}
