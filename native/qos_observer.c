#include <dlfcn.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <time.h>
static volatile sig_atomic_t stopped;
static volatile unsigned packets;
static void stop(int sig){(void)sig;stopped=1;}
static void notify(int sig){(void)sig;}
static void expired(int sig){(void)sig;_exit(124);}
static void log_packet(unsigned char *data,int size){
 if(!data||stopped||size<12||size>4096||packets>=32)return;
 if(size>=16&&data[0]==0x10&&data[1]==0){data+=4;size-=4;}
 unsigned length=data[0]|data[1]<<8,code=data[2]|data[3]<<8;
 if(length!=(unsigned)size||(code!=0xb0e5&&code!=0xb0e2&&code!=0xb0ec))return;
 if(code==0xb0ec){if(size<23||data[12]!=1||data[16]!=7||data[17]!=0x42)return;unsigned at=21u+data[20];if(at+2>(unsigned)size)return;unsigned n=(unsigned)data[at]*256+data[at+1];at+=2;if(n<3||at+n>(unsigned)size)return;packets++;flockfile(stdout);printf("ESM ");for(unsigned j=0;j<n;j++)printf("%02x",data[at+j]);puts("");funlockfile(stdout);return;}
 packets++;flockfile(stdout);printf("LOG %04x ",code);for(int i=0;i<size;i++)printf("%02x",data[i]);puts("");funlockfile(stdout);
}
static unsigned char demand_reply[4096];
static void on_reply(unsigned char *p,int n,void *ctx){(void)ctx;printf("DEMAND_REPLY ");for(int i=0;i<n&&i<64;i++)printf("%02x",p[i]);puts("");}
int main(int argc,char **argv){
 int proc=argc>1?atoi(argv[1]):0;if(proc<0||proc>1)return 2;
 setbuf(stdout,NULL);signal(SIGUSR1,notify);signal(SIGTERM,stop);signal(SIGINT,stop);signal(SIGALRM,expired);int seconds=argc>3?atoi(argv[3]):10;if(seconds<1||seconds>90)seconds=10;alarm(seconds+15);
 void *lib=dlopen("/usr/lib/libdiag.so.1",RTLD_NOW|RTLD_LOCAL);if(!lib){printf("library: %s\n",dlerror());return 1;}
 int (*init)(void*)=dlsym(lib,"Diag_LSM_Init");int (*deinit)(void)=dlsym(lib,"Diag_LSM_DeInit");
 int (*support)(int,uint16_t*)=dlsym(lib,"diag_get_dci_support_list_proc");
 int (*reg)(int*,uint16_t*,int,void*)=dlsym(lib,"diag_register_dci_client");
 int (*stream)(int,void(*)(unsigned char*,int),void(*)(unsigned char*,int))=dlsym(lib,"diag_register_dci_stream_proc");
 int (*config)(int,int,uint16_t*,int)=dlsym(lib,"diag_log_stream_config");
 int (*release)(int*)=dlsym(lib,"diag_release_dci_client");
 int (*demand)(int,unsigned char*,int,unsigned char*,int,void(*)(unsigned char*,int,void*),void*)=dlsym(lib,"diag_send_dci_async_req");
 if(!init||!deinit||!reg||!stream||!config||!release){puts("api unavailable");return 1;}
 if(!init(NULL)){puts("diag init failed");return 1;}
 uint16_t peripherals=0,codes[]={0xb0e5,0xb0e2,0xb0ec};int client=-1,sig=SIGUSR1;
 if(support){int status=support(proc,&peripherals);printf("support proc=%d status=%d mask=%04x\n",proc,status,peripherals);if(status!=1001||!peripherals){deinit();return 1;}}
 int result=reg(&client,&peripherals,proc,&sig);printf("register status=%d client=%d\n",result,client);
 int enabled=0;
 if(result==1001&&client>0){result=stream(client,log_packet,NULL);printf("stream status=%d\n",result);if(result==1001){result=config(client,1,codes,3);printf("subscribe status=%d\n",result);enabled=result==1001;if(enabled){if(argc>2&&strcmp(argv[2],"demand")==0&&demand){usleep(500000);unsigned char query[]={0x78,0xe5,0xb0};printf("demand status=%d\n",demand(client,query,3,demand_reply,sizeof demand_reply,on_reply,NULL));}for(int i=0;i<seconds*10&&!stopped;i++)usleep(100000);}}}
 if(enabled)printf("unsubscribe status=%d\n",config(client,0,codes,3));
 if(client>0)printf("release status=%d\n",release(&client));printf("deinit=%d packets=%u\n",deinit(),packets);return 0;
}
