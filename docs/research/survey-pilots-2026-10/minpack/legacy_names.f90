! Shim: the F77 external names the legacy drivers link against, forwarded to
! fortran-lang/minpack's module procedures (pilot only).
double precision function dpmpar(i)
  use minpack_module, only: m => dpmpar
  integer, intent(in) :: i
  dpmpar = m(i)
end function
double precision function enorm(n, x)
  use minpack_module, only: m => enorm
  integer, intent(in) :: n
  double precision, intent(in) :: x(n)
  enorm = m(n, x)
end function
subroutine hybrd1(fcn, n, x, fvec, tol, info, wa, lwa)
  use minpack_module, only: m => hybrd1, func
  procedure(func) :: fcn
  integer :: n, info, lwa
  double precision :: x(n), fvec(n), tol, wa(lwa)
  call m(fcn, n, x, fvec, tol, info, wa, lwa)
end subroutine
subroutine hybrj1(fcn, n, x, fvec, fjac, ldfjac, tol, info, wa, lwa)
  use minpack_module, only: m => hybrj1, fcn_hybrj
  procedure(fcn_hybrj) :: fcn
  integer :: n, ldfjac, info, lwa
  double precision :: x(n), fvec(n), fjac(ldfjac, n), tol, wa(lwa)
  call m(fcn, n, x, fvec, fjac, ldfjac, tol, info, wa, lwa)
end subroutine
subroutine lmder1(fcn, mm, n, x, fvec, fjac, ldfjac, tol, info, ipvt, wa, lwa)
  use minpack_module, only: m => lmder1, fcn_lmder
  procedure(fcn_lmder) :: fcn
  integer :: mm, n, ldfjac, info, lwa, ipvt(n)
  double precision :: x(n), fvec(mm), fjac(ldfjac, n), tol, wa(lwa)
  call m(fcn, mm, n, x, fvec, fjac, ldfjac, tol, info, ipvt, wa, lwa)
end subroutine
subroutine lmdif1(fcn, mm, n, x, fvec, tol, info, iwa, wa, lwa)
  use minpack_module, only: m => lmdif1, func2
  procedure(func2) :: fcn
  integer :: mm, n, info, lwa, iwa(n)
  double precision :: x(n), fvec(mm), tol, wa(lwa)
  call m(fcn, mm, n, x, fvec, tol, info, iwa, wa, lwa)
end subroutine
subroutine lmstr1(fcn, mm, n, x, fvec, fjac, ldfjac, tol, info, ipvt, wa, lwa)
  use minpack_module, only: m => lmstr1, fcn_lmstr
  procedure(fcn_lmstr) :: fcn
  integer :: mm, n, ldfjac, info, lwa, ipvt(n)
  double precision :: x(n), fvec(mm), fjac(ldfjac, n), tol, wa(lwa)
  call m(fcn, mm, n, x, fvec, fjac, ldfjac, tol, info, ipvt, wa, lwa)
end subroutine
subroutine chkder(mm, n, x, fvec, fjac, ldfjac, xp, fvecp, mode, err)
  use minpack_module, only: m => chkder
  integer :: mm, n, ldfjac, mode
  double precision :: x(n), fvec(mm), fjac(ldfjac, n), xp(n), fvecp(mm), err(mm)
  call m(mm, n, x, fvec, fjac, ldfjac, xp, fvecp, mode, err)
end subroutine
