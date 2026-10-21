C     DRV: a FORTRAN 77 driver with no PROGRAM statement.
      INTEGER NREAD, N
      COMMON /STATE/ N, X(100)
      DATA NREAD /5/
      READ (NREAD, 50) N
      CALL INIT(N)
      STOP
   50 FORMAT (I5)
      END
