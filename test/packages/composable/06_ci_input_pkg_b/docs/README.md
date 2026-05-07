# CI Input Package B

`type: input` package consumed by [`07_ci_composable_dual_input`](../07_ci_composable_dual_input/). Both this package and [`05_ci_input_pkg_a`](../05_ci_input_pkg_a/) declare `input: logfile`, so the composable build must assign each a unique `name` qualifier to satisfy package-spec SVR00010.
