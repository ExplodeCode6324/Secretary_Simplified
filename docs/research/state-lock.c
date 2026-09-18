/* Prototype for native/state-lock.c. Caller must own a private state directory. */
#include <sys/file.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv) {
    if (argc < 3) { fprintf(stderr, "usage: state-lock LOCK_FILE PROGRAM [ARGS...]\n"); return 64; }
    int fd = open(argv[1], O_CREAT | O_RDWR | O_NOFOLLOW, 0600);
    if (fd < 0) { perror("open lock"); return 74; }
    struct stat st;
    if (fstat(fd, &st) || !S_ISREG(st.st_mode) || st.st_uid != geteuid()
        || st.st_nlink != 1 || (st.st_mode & 0077)) {
        fprintf(stderr, "unsafe lock file\n"); close(fd); return 77;
    }
    if (flock(fd, LOCK_EX | LOCK_NB)) { fprintf(stderr, "state directory already owned\n"); close(fd); return 75; }
    int flags = fcntl(fd, F_GETFD);
    if (flags < 0 || fcntl(fd, F_SETFD, flags & ~FD_CLOEXEC) < 0) {
        perror("lock descriptor"); close(fd); return 74;
    }
    char value[32]; snprintf(value, sizeof(value), "%d", fd);
    if (setenv("SECRETARY_LOCK_FD", value, 1)) { perror("setenv"); close(fd); return 74; }
    execvp(argv[2], &argv[2]);
    perror("exec"); close(fd); return 74;
}
