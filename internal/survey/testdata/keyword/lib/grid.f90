! A free-form Fortran 90 module.
module grid
  implicit none
  real(8), allocatable :: cells(:,:)
  type :: point
    real(8) :: x, y
  end type point
contains
  subroutine resize(n)
    integer, intent(in) :: n
    allocate(cells(0:n, -1:n))
    call clear()
  end subroutine resize
  pure function area(p) result(a)
    type(point), intent(in) :: p
    real(8) :: a
    a = p%x * p%y
  end function area
  subroutine clear()
    cells = 0
  end subroutine
end module grid

program main
  use grid
  use iso_fortran_env, only: real64
  implicit none
  call resize(10)
  print *, area(point(1d0, 2d0)) ! a comment with call fake()
end program main
