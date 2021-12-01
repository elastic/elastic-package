package shell

const BashInitCode = ShInitCode

const FishInitCode = `set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;
`

const ShInitCode = `export %s=%s
export %s=%s
export %s=%s
export %s=%s
export %s=%s
`

const ZshInitCode = ShInitCode
