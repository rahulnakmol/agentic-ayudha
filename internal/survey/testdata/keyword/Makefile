FC = gfortran
# FFLAGS = -ffast-math
FFLAGS = -O2 -fno-automatic # was -Ofast
solve: src/solver.f
	$(FC) $(FFLAGS) -o solve src/solver.f
