# Survey: minpack

MINPACK is a library of FORTRAN 77 subroutines for solving systems of nonlinear equations and nonlinear least-squares problems, in five algorithmic paths that each pair a core routine with an easy-to-use driver, plus a Jacobian checker. This checkout keeps the original fixed-form code to test the LFortran compiler; modern development moved to fortran-lang/minpack. Who calls it in the person's code, through which entry points, and how exact 'the same results' must be are unknown and asked. Running it showed that its results depend on how it is built: the project's own Release flags change the output of all six legacy test drivers.

Asked for: "This is the original MINPACK, fixed-form Fortran 77, that our numerical code calls; it has a readme and no other product docs. Tell us what it does, which traps a modern port must check, what we should not carry over, and how we would move it to modern Fortran (free form, modules) so that every caller gets the same results.". Indexed at commit `12f30bc2d697`: 82 files, fortran (10825 lines), cmake (18 lines).

Each claim says how it is known: **observed** (the code shows it), **stated** (a document or comment says it), **inferred** (reasoned from the cited code), **confirmed** (the person said it), or **unknown** (a question asks it). Citations are index fact IDs or `path:line` ranges at the commit above.

## Limits of this survey

- Calls are resolved by name only: 18 of 115 call edges name more than one definition.
- Dynamic dispatch the index cannot follow: `ex/hybdrv.f:30`, `ex/hyjdrv.f:31`, `ex/lmddrv.f:31`, `ex/lmfdrv.f:30`, `ex/lmsdrv.f:31`, `ex/ucodrv.f:30`, `src/hybrd.f:8`, `src/hybrd1.f:5`, `src/hybrj1.f:5`, `src/lmder1.f:7`, `src/lmdif.f:9`, `src/lmdif1.f:6`, `src/lmstr1.f:7`.
- Files with no language the index knows: (none) (23), .yml (1), .gitignore (1).
- Product claims come from documents, inference, or the person; code alone never shows who uses the software or why.

## Engineering

### Architecture

- **E1** (observed) A flat library of independent subroutines with no modules or shared state: each easy-to-use driver (hybrd1, hybrj1, lmdif1, lmder1, lmstr1) checks its inputs, sets default tolerances and limits, and calls its core routine, which calls shared linear-algebra helpers (qrfac, qrsolv, lmpar, dogleg, enorm). `sym:src/lmder1.f#lmder1`, `src/lmder1.f:129-151`, `sym:src/lmder.f#lmder`, `doc:README.rst`.

### Data

- **E8** (observed) dpmpar's comment says machine constants come from DATA statements activated per machine, but its code returns the intrinsics epsilon, tiny, and huge of a double: the comment is stale and the constants follow the compiler. `src/dpmpar.f:7-11`, `src/dpmpar.f:35-40`.
- **E9** (observed) enorm avoids overflow and underflow with two hard-coded constants, rdwarf 3.834d-20 and rgiant 1.304d19, rather than machine constants. `src/enorm.f:44`, `src/enorm.f:16-22`.

### Interfaces

- **E2** (observed) lmder1 sets ftol and xtol to the caller's tol, gtol to zero, maxfev to 100\*(n+1), mode 1, factor 100, and nprint 0, rejects m \< n, ldfjac \< m, tol \< 0, or lwa \< 5\*n+m with info 0, and maps the core's info 8 to 4. `src/lmder1.f:131-151`.
- **E3** (observed) The caller passes the user function as an EXTERNAL procedure (fcn) that receives iflag and fills the function values or the Jacobian; the calling sequence has no argument for the caller's own data. `trap:fortran-external:src/lmder1.f:7`, `src/lmder1.f:1-7`, `sym:ex/lmddrv.f#fcn`.
- **E4** (observed) The caller allocates every work array and passes its length (wa and lwa for lmder1, at least 5\*n+m); lmder1 slices wa into the core routine's diag, qtf, and four work vectors. `src/lmder1.f:115-117`, `src/lmder1.f:147-149`.
- **E5** (observed) src/minpack.f90 is a module named minpack holding explicit interfaces for dpmpar, enorm, hybrd, hybrd1, lmder1, lmdif1, and chkder only; the examples call the library through it (use minpack, only: …), while the ex/ drivers call the F77 names directly. `sym:src/minpack.f90#minpack`, `src/minpack.f90:1-89`, `examples/example_lmder1.f90:46`, `src/lmder1.f:1`.

### Build

- **E6** (observed) CMake builds one library target from every file in src/, including the test-problem routines, with -std=legacy; an unset build type becomes Debug; UserOverride.cmake sets Release to -O3 -march=native -ffast-math -funroll-loops for gfortran. fpm builds the same sources with only the examples as executables. `build:src/CMakeLists.txt#minpack`, `src/CMakeLists.txt:1-13`, `CMakeLists.txt:10-14`, `cmake/UserOverride.cmake:8-12`, `fpm.toml:12-21`.

### Dependencies

- **E13** (inferred) The library calls nothing outside itself and the Fortran intrinsics; the build declares no dependency. `src/CMakeLists.txt:1-13`, `fpm.toml:1-15`. Reasoning: The index records no dependency facts and no file, network, or environment surface in src/; the only I/O is in the drivers and examples.

### Tests

- **E7** (observed) There is no automated test: the ex/ drivers are main programs that read problem numbers, sizes, and repetitions from unit 5 (the netlib data files ex/file21-23), run one solver per problem from a standard starting point scaled by 1, 10, and 100, and print norms, evaluation counts, the exit parameter, and the solution to unit 6; nothing compares the output with expected values. `entry:ex/lmddrv.f:25`, `ex/lmddrv.f:37-77`, `io:unit-io:ex/lmddrv.f:45`, `ex/file22:1-4`.
- **E12** (observed) No driver calls covar: running the six drivers under coverage never reaches it. `probe:unreached:src/covar.f#covar`, `sym:src/covar.f#covar`.

### Risks

- **E10** (observed) The same code built two other ways prints different results: the Release flags change the output of all six drivers (a different solution vector for lmder1, one more function evaluation for lmdif1), and an -O1 build with -fcheck=all and the address and undefined-behaviour sanitisers ends lmder1 and lmdif1 with a different exit parameter; that build reported no runtime error. `probe:check-differs:lmder1`, `probe:check-differs:lmdif1`, `probe:check-differs:hybrd1`, `probe:check-differs:chkder`.
- **E11** (observed) Three files define a subroutine named initpt (hybipt.f, lmdipt.f, ocpipt.f) and two define hesfcn (hesfcn.f, lhesfcn.f), and the CMake library target lists all of them, so which one a program gets depends on the linker. `sym:src/hybipt.f#initpt`, `sym:src/lmdipt.f#initpt`, `sym:src/ocpipt.f#initpt`, `sym:src/hesfcn.f#hesfcn`, `sym:src/lhesfcn.f#hesfcn`, `src/CMakeLists.txt:1-13`.

## Capability map

| ID | Capability | What it does | Basis | Evidence |
|---|---|---|---|---|
| C1 | Solve a system of nonlinear equations from the functions alone | Find a zero of n equations in n unknowns when the caller supplies only the functions; the Jacobian is approximated by forward differences (hybrd, hybrd1). | observed | `sym:src/hybrd1.f#hybrd1`, `doc:README.rst` |
| C2 | Solve a system of nonlinear equations with a supplied Jacobian | The same, with the caller's analytic Jacobian (hybrj, hybrj1). | observed | `sym:src/hybrj1.f#hybrj1` |
| C3 | Fit a model by nonlinear least squares | Minimise the sum of squares of m functions in n unknowns by Levenberg-Marquardt, with a finite-difference Jacobian (lmdif1), an analytic one (lmder1), or one row at a time to save memory (lmstr1). | observed | `sym:src/lmdif1.f#lmdif1`, `sym:src/lmder1.f#lmder1`, `sym:src/lmstr1.f#lmstr1` |
| C4 | Check a Jacobian against its functions | Report, per function, how consistent a supplied Jacobian is with finite differences of the functions (chkder). | observed | `sym:src/chkder.f#chkder` |
| C5 | Estimate the covariance of a fit | Compute the covariance matrix from the factorisation a least-squares solver returns (covar); nothing in this checkout exercises it. | observed | `sym:src/covar.f#covar`, `probe:unreached:src/covar.f#covar` |

## Product

- **P1** (stated) This repository keeps MINPACK in its original fixed form to test the LFortran compiler on F77-style code; modern development moved to fortran-lang/minpack. `README.rst:1-5`.
- **P2** (stated) It serves people who need to solve nonlinear equations or nonlinear least-squares problems, with or without an analytic Jacobian. `README.rst:20-27`.
- **P3** (unknown, Q1) Which of the person's programs call it, and through which entry points, is not in the code.
- **P4** (unknown, Q2) How exact 'the same results' must be, and on which compiler and flags, is not in the code.

## Traps checked

Places where the stack's semantics differ from what a port would assume, each checked in the code (references/traps.md).

- **T1** (observed) Yes: the Release flags (-O3 -march=native -ffast-math) change the output of all six drivers, and the default build type is Debug, so two builds from the same CMake files disagree. `trap:build-semantic-flags:cmake/UserOverride.cmake:11`, `CMakeLists.txt:10-14`, `probe:check-differs:lmder1`, `probe:check-differs:lmstr1`. Rule `build-semantic-flags`: Do the build's flags change results, and which build were the goldens captured with?
- **T2** (observed) Only the drivers use COMMON: /refnum/ carries the problem number and the evaluation counters from the driver to its fcn callback, because fcn's calling sequence has no slot for caller data. The library in src/ has none. `trap:fortran-common:ex/lmddrv.f:32`, `ex/lmddrv.f:32`, `ex/lmddrv.f:114-119`. Rule `fortran-common`: What state do COMMON blocks share, and between which units?
- **T3** (inferred) No unit declares IMPLICIT NONE, but none relies on implicit typing: every file in src/ and ex/ compiles with gfortran -fimplicit-none. `trap:fortran-implicit-typing:src/lmder1.f:1`, `src/lmder1.f:3-6`, `src/lmder1.f:129-130`. Reasoning: Compiling each src/\*.f and ex/\*drv.f with gfortran -std=legacy -fimplicit-none -fsyntax-only reported no undeclared name; the declarations at the cited lines show the pattern. Rule `fortran-implicit-typing`: Does any unit rely on implicit typing (an undeclared name starting I-N is INTEGER)?
- **T4** (observed) enorm hard-codes rdwarf and rgiant, while dpmpar returns the compiler's epsilon, tiny, and huge despite a comment describing per-machine DATA statements, and the drivers derive their tolerance from it (tol = sqrt(dpmpar(1))); a port keeps enorm's two literals and dpmpar's values to keep the results. `trap:fortran-machine-constants:src/dpmpar.f:1`, `src/enorm.f:44`, `src/dpmpar.f:7-11`, `src/dpmpar.f:35-38`, `ex/lmddrv.f:40`. Rule `fortran-machine-constants`: Which machine constants does it hard-code, and does the port keep their values?
- **T5** (observed) Only the drivers' FORMAT statements use Hollerith text (8h problem, 33h initial l2 norm …); it is the text of the output the goldens capture, so a port of the drivers must print the same characters. `trap:fortran-hollerith:ex/lmddrv.f:80`, `ex/lmddrv.f:79-89`. Rule `fortran-hollerith`: Where are Hollerith constants, and does any output depend on them?
- **T6** (observed) No: fcn is declared EXTERNAL with no interface in the library, so a callback with the wrong argument list compiles and fails at run time; src/minpack.f90 gives interfaces only for the routines it lists. `trap:fortran-external:src/lmder1.f:7`, `src/minpack.f90:50-63`. Rule `fortran-external`: Is each EXTERNAL procedure's interface checked anywhere?
- **T7** (observed) Jacobians are column-major with an explicit leading dimension (fjac(ldfjac,n)); the drivers pass ldfjac 65 for m up to 65, so the modern interface must keep ldfjac rather than assume a contiguous m-by-n array. `src/lmder1.f:6`, `ex/lmddrv.f:29`, `ex/lmddrv.f:41`. Rule `stack`: Are arrays column-major with explicit leading dimensions, and does any caller pass a leading dimension larger than the row count?
- **T8** (observed) In the routines read, DATA initialises constants that are never assigned (factor and zero in lmder1; one, zero, rdwarf, rgiant in enorm), so the implicit SAVE carries no state between calls there. `trap:fortran-save-data:src/lmder1.f:131`, `src/lmder1.f:131-151`, `src/enorm.f:44-51`. Rule `fortran-save-data`: Does any DATA-initialised local change between calls (implicit SAVE)?

## Questions for the person

| ID | Question | Why it matters | Default | Status |
|---|---|---|---|---|
| Q1 | Which of your programs call MINPACK, through which entry points (the F77 names such as lmder1, the minpack interface module, or both), and can they be recompiled? | It decides whether the modern library must keep the F77 external names and argument lists (a shim) or only the module interface (E5, K2, K3). | Keep both through a thin shim until every caller uses the module, since recompiling every caller is not known to be possible. | open |
| Q2 | Must results be bit-identical, and on which compiler and flags, or is agreement within a tolerance enough? | The same code gives different results under the project's own Release flags and under an -O1 checker build (E10), so 'the same results' needs a reference build or a tolerance (K1, G1). | Bit-identical against a legacy build with a pinned gfortran at -O2 and no fast-math, since that build matched the modern library exactly in the pilot. | open |
| Q3 | Do any callers read the work array wa after a call, or size it beyond the documented minimum on purpose? | Replacing caller-allocated work arrays (N2) changes the calling sequence; a caller that reads wa (lmder1 passes wa(1:n) as diag) would break. | Keep wa and lwa in the shim and allocate internally in the new module API. | open |
| Q4 | Does any caller use the test-problem routines (initpt, ssqfcn, vecfcn, hesfcn, …) from the library? | They are compiled into the library with duplicate names (E11); moving them to the tests (N3) removes them from the library. | Move them to the test suite, since only the drivers call them in this checkout. | open |
| Q5 | Which flags will the modern library ship with, and may they include -ffast-math or -march=native? | Those flags change results (E10, T1); keeping them as the Release default would carry a source of differences into the modern version (N4). | Ship with IEEE-preserving flags (no -ffast-math, a portable -march) and offer fast flags only as an opt-in build. | open |

## Do not carry over

What the modern version deliberately does not reproduce. The person decides each; a confirmed entry becomes a Decision that constrains the Stories porting its code.

| ID | Feature | Evidence | Risk of copying it | Replacement | Decision |
|---|---|---|---|---|---|
| N1 | Caller state passed to the callback through a COMMON block | `ex/lmddrv.f:32`, `ex/lmddrv.f:114-119`; T2 | Global mutable state makes two solves in one program, or a solve on two threads, share counters and problem numbers. | Give the callback the caller's data: a module procedure with a derived-type argument or an internal procedure, as fortran-lang/minpack's interfaces allow. | proposed (Q1) |
| N2 | Caller-allocated work arrays with size rules (wa, lwa ≥ 5\*n+m) | `src/lmder1.f:115-117`, `src/lmder1.f:136-137` | A short work array fails with info 0 at run time; a wrong slice silently corrupts results. | Allocate work space inside the modern API and keep the legacy calling sequence only in the shim (SH1). | proposed (Q3) |
| N3 | Test-problem routines with duplicate names compiled into the library | `sym:src/hybipt.f#initpt`, `sym:src/lmdipt.f#initpt`, `sym:src/hesfcn.f#hesfcn`, `src/CMakeLists.txt:1-13` | Which initpt or hesfcn a program links is decided by the linker, not the code. | Move the test problems out of the library into the test suite, one name per routine. | proposed (Q4) |
| N4 | -ffast-math and -march=native as the Release default | `cmake/UserOverride.cmake:11`, `probe:check-differs:lmder1`; T1 | Results change with the flags and with the machine that builds them (E10). | IEEE-preserving release flags with a portable target; fast flags only as an opt-in build that is not the parity reference. | proposed (Q5) |

## Compatibility contract

Every interface others depend on, at the level the person decides: **exact** (must match), **differ** (may differ, with the recorded decision), **drop** (removed, with a deprecation path), or **undecided** (a question asks).

| ID | Kind | Interface | Level | Decided by | Proof | Who depends on it |
|---|---|---|---|---|---|---|
| K1 | api | The numbers every solver returns: x, fvec, info, and the evaluation counts, for the same inputs `src/lmder1.f:86-106`, `src/lmder1.f:147-151` | exact | request | G1, G2 | the person's numerical code (which programs: Q1) |
| K2 | api | The F77 external names and argument lists (lmder1(fcn,m,n,x,fvec,fjac,ldfjac,tol,info,ipvt,wa,lwa) and the others) `src/lmder1.f:1-7`, `src/hybrd1.f:1-5` | undecided | Q1 | G1 | callers linking the library without a module (unknown which: Q1) |
| K3 | api | The minpack interface module and its names (use minpack, only: lmder1, …) `src/minpack.f90:1-89`, `examples/example_lmder1.f90:46` | undecided | Q1 | G2 | the examples; other callers unknown (Q1) |

## Modernisation path: Modern Fortran (free form, modules)

Target from the request. Characterise first: capture the six drivers' and five examples' output from a legacy build with pinned flags (Q2's default). Then port the library to free-form modules routine by routine behind the same algorithms and constants, keep the F77 names through a shim while callers move (Q1), and shadow the legacy and modern libraries under the same drivers until every case matches. fortran-lang/minpack is a reference port to compare with. (`entry:ex/lmddrv.f:25`, `README.rst:1-5`)

### Steps

1. **M1** Pin the reference build and capture golden output from the drivers and examples. Replaces `mod:ex`, `mod:examples`, `mod:cmake`. Rests on E7, E10. Does not carry over N1, N4. Parity: G1, G2.
2. **M2** Port the linear-algebra helpers and enorm/dpmpar to a module with explicit interfaces. Replaces `mod:src#1`, `mod:src#3`, `mod:src#4`. After M1. Rests on E1, E8, E9. Does not carry over N3. Parity: G1.
3. **M3** Port the five solver paths and chkder, allocating work space internally. Replaces `mod:src#2`, `mod:src#3`. After M2. Rests on E2, E3, E4. Does not carry over N2, N3. Parity: G1, G2.
4. **M4** Keep the F77 names through a shim and shadow both libraries under the drivers. Replaces `mod:ex`. After M3. Rests on E5. Does not carry over N1. Parity: G1.

### Parity case sets

- **G1** The six netlib drivers on their data files (hybrd1, hybrj1, lmder1, lmdif1, lmstr1, chkder). Inputs from `entry:ex/lmddrv.f:25`, `entry:ex/hybdrv.f:25`, `entry:ex/chkdrv.f:20`, `ex/file21:1-23`, `ex/file22:1-29`, `ex/file23:1-15`; compared exact stdout and exit code, legacy and modern built with the same compiler and flags.
- **G2** The five examples through the minpack module. Inputs from `entry:examples/example_lmder1.f90:45`, `entry:examples/example_hybrd.f90:6`, `entry:examples/example_primes.f90:67`; compared exact stdout and exit code.

### Coexistence before cut-over: shadow

Link the legacy drivers to the legacy library and, through the shim, to the modern one; run both on G1 and on the callers' own inputs and report every difference. Compared on G1. Cut over when Every G1 and G2 case matches on the pinned build, and the callers' inputs show no difference for a release cycle. Rollback: Relink the callers against the legacy library, which stays built until the shim is removed.

### Compatibility shims

- **SH1** for K2: The F77 external names and argument lists, forwarding to the module procedures, turned on by a legacy\_names source file in the library that defines each F77 routine and calls the module procedure. Removal: When every caller named in Q1 uses the module, deprecate the shim for one release, then delete it..

### Modernisation risks

- **R1** (observed) Exact parity holds only between builds with the same compiler and flags: the Release flags and an -O1 checker build already change the legacy results. `probe:check-differs:lmder1`, `probe:check-differs:hybrj1`.
- **R2** (inferred) Module procedures get module-qualified link names, so callers that call the F77 names will not link against a module-only library without a shim; check every caller's link line. `src/minpack.f90:1-3`, `ex/lmddrv.f:56-57`. Reasoning: The drivers call lmder1 as an external procedure; a procedure inside a module is not an external procedure of that name.

## Coverage

- Modules cited: 6 of 7. Not cited: `mod:src#4`.
- Entry points cited: 6 of 12.
- Known-issue markers cited: 0 of 0.
- Trap facts checked: 7 of 209. Not checked: `trap:build-semantic-flags:src/CMakeLists.txt:12`, `trap:fortran-common:ex/hybdrv.f:31`, `trap:fortran-common:ex/hyjdrv.f:32`, `trap:fortran-common:ex/lmfdrv.f:31`, `trap:fortran-common:ex/lmsdrv.f:32`, `trap:fortran-common:ex/ucodrv.f:31`, `trap:fortran-external:ex/hybdrv.f:30`, `trap:fortran-external:ex/hyjdrv.f:31`, `trap:fortran-external:ex/lmddrv.f:31`, `trap:fortran-external:ex/lmfdrv.f:30`, `trap:fortran-external:ex/lmsdrv.f:31`, `trap:fortran-external:ex/ucodrv.f:30`, `trap:fortran-external:src/hybrd.f:8`, `trap:fortran-external:src/hybrd1.f:5`, `trap:fortran-external:src/hybrj1.f:5`, `trap:fortran-external:src/lmdif.f:9`, `trap:fortran-external:src/lmdif1.f:6`, `trap:fortran-external:src/lmstr1.f:7`, `trap:fortran-format:ex/chkdrv.f:75`, `trap:fortran-format:ex/hybdrv.f:71` and 182 more (the index lists them).
- Open questions: 5; unknown claims: 2.
