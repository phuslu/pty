//go:build aix || solaris

#include "textflag.h"

#ifdef GOOS_aix
#ifdef GOARCH_ppc64
TEXT ·aixcall6(SB),NOSPLIT,$0-88
	JMP syscall·syscall6(SB)
#endif
#endif

#ifdef GOOS_solaris
#ifdef GOARCH_amd64
TEXT ·sysvicall6(SB),NOSPLIT,$0-88
	JMP syscall·sysvicall6(SB)
#endif
#endif
