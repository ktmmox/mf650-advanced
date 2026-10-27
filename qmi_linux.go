//go:build linux

package main
import("encoding/binary";"fmt";"syscall";"time";"unsafe")
type qrtrAddr struct{Family uint16; Pad uint16; Node,Port uint32}
var le=binary.LittleEndian
func qrtrSend(fd int,b []byte,a *qrtrAddr)error{_,_,e:=syscall.Syscall6(syscall.SYS_SENDTO,uintptr(fd),uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),0,uintptr(unsafe.Pointer(a)),12);if e!=0{return e};return nil}
func qrtrRecv(fd int)([]byte,error){b:=make([]byte,65536);n,_,e:=syscall.Syscall6(syscall.SYS_RECVFROM,uintptr(fd),uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),0,0,0);if e!=0{return nil,e};return b[:n],nil}
func qrtrOpen()(int,error){fd,e:=syscall.Socket(42,syscall.SOCK_DGRAM,0);if e==nil{e=syscall.SetsockoptTimeval(fd,syscall.SOL_SOCKET,syscall.SO_RCVTIMEO,&syscall.Timeval{Sec:2});syscall.SetsockoptTimeval(fd,syscall.SOL_SOCKET,syscall.SO_SNDTIMEO,&syscall.Timeval{Sec:2})};if e!=nil&&fd>=0{syscall.Close(fd)};return fd,e}
func qrtrLookup(svc uint32)(qrtrAddr,error){fd,e:=qrtrOpen();if e!=nil{return qrtrAddr{},e};defer syscall.Close(fd);self:=qrtrAddr{};n:=uint32(12);syscall.Syscall(syscall.SYS_GETSOCKNAME,uintptr(fd),uintptr(unsafe.Pointer(&self)),uintptr(unsafe.Pointer(&n)));ctrl:=qrtrAddr{Family:42,Node:self.Node,Port:0xfffffffe};if ctrl.Node==0{ctrl.Node=1};b:=make([]byte,20);le.PutUint32(b,10);le.PutUint32(b[4:],svc);if e=qrtrSend(fd,b,&ctrl);e!=nil{return qrtrAddr{},e};end:=time.Now().Add(3*time.Second);for time.Now().Before(end){b,e=qrtrRecv(fd);if e!=nil{return qrtrAddr{},e};if len(b)>=20&&le.Uint32(b)==4&&le.Uint32(b[4:])==svc{return qrtrAddr{Family:42,Node:le.Uint32(b[12:]),Port:le.Uint32(b[16:])},nil}};return qrtrAddr{},fmt.Errorf("service absent")}

// Each independent socket is its own QMI client. Only NAS operations below are exposed.
func nasQuery(id uint16,payload []byte)(map[byte][]byte,error){
 switch id {case 0x43,0xd7,0x10f,0xd8:default:return nil,fmt.Errorf("不允许的 QMI 请求")}
 a,e:=qrtrLookup(3);if e!=nil{return nil,fmt.Errorf("QMI 服务发现失败：%v",e)};fd,e:=qrtrOpen();if e!=nil{return nil,e};defer syscall.Close(fd)
 b:=make([]byte,7+len(payload));le.PutUint16(b[1:],1);le.PutUint16(b[3:],id);le.PutUint16(b[5:],uint16(len(payload)));copy(b[7:],payload)
 if e=qrtrSend(fd,b,&a);e!=nil{return nil,e};end:=time.Now().Add(4*time.Second)
 for time.Now().Before(end){b,e=qrtrRecv(fd);if e!=nil{return nil,e};if len(b)<7||b[0]!=2||le.Uint16(b[1:])!=1||le.Uint16(b[3:])!=id{continue};return parseQMIFrame(b,id)}
 return nil,fmt.Errorf("QMI 查询超时")
}
