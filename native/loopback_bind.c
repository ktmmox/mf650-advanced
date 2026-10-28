#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <netinet/in.h>
#include <string.h>
#include <sys/socket.h>

/* Scope this preload to the compatibility httpd, never the main console.
 * Keep its port-80 listener local while the new console owns the LAN address. */
int bind(int fd, const struct sockaddr *addr, socklen_t len) {
    typedef int (*bind_fn)(int, const struct sockaddr *, socklen_t);
    bind_fn next = (bind_fn)dlsym(RTLD_NEXT, "bind");
    if (!next) { errno = ENOSYS; return -1; }
    if (addr && addr->sa_family == AF_INET && len >= sizeof(struct sockaddr_in)) {
        struct sockaddr_in local;
        memcpy(&local, addr, sizeof(local));
        if (ntohs(local.sin_port) == 80) {
            local.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
            return next(fd, (const struct sockaddr *)&local, sizeof(local));
        }
    }
    if (addr && addr->sa_family == AF_INET6 && len >= sizeof(struct sockaddr_in6)) {
        struct sockaddr_in6 local;
        memcpy(&local, addr, sizeof(local));
        if (ntohs(local.sin6_port) == 80) {
            local.sin6_addr = in6addr_loopback;
            local.sin6_scope_id = 0;
            return next(fd, (const struct sockaddr *)&local, sizeof(local));
        }
    }
    return next(fd, addr, len);
}
