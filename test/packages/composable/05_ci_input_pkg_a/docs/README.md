# CI Input Package A

`type: input` package consumed by [`07_ci_composable_dual_input`](../07_ci_composable_dual_input/). Both this package and [`06_ci_input_pkg_b`](../06_ci_input_pkg_b/) declare `input: logfile`, so the composable build must assign each a unique `name` qualifier to satisfy package-spec SVR00010.
