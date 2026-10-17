C     SOLVER: a fixed-form FORTRAN 77 unit with a COMMON block.
      PROGRAM SOLVE
      COMMON /STATE/ N, X(100)
      INCLUDE 'params.inc'
      CALL INIT(N)
      CALL STEP(X, N)
      Y = FNORM(X, N)
      WRITE(*,*) Y
      STOP
      END
C
      SUBROUTINE INIT(M)
      COMMON /STATE/ N, X(100)
      EQUIVALENCE (X(1), Z)
      DO 10 I = 1, M
         X(I) = 0.0
   10 CONTINUE
      RETURN
      END
      SUBROUTINE STEP(V,
     &                K)
      IMPLICIT NONE
      INTEGER K
      REAL V(*)
      CALL INIT(K)                                                      00001234
      END
      REAL FUNCTION FNORM(V, K)
      IMPLICIT NONE
      INTEGER K
      REAL V(K)
      FNORM = SQRT(V(1))
      END
