#include <stdio.h>
#include <unistd.h>
#include <sys/utsname.h>

int main(int argc, char **argv)
{
	struct utsname u;

	uname(&u);
	printf("hello from %s (pid %d) on %s %s %s\n", argv[0], getpid(), u.sysname, u.release, u.machine);

	for (int i = 1; i < argc; i++)
		printf("argv[%d] = %s\n", i, argv[i]);

	return 0;
}
