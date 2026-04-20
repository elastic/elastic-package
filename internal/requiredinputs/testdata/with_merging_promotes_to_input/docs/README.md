# Variable Merging - Promotes to Input Var

Test fixture: composable package whose policy template declares a "paths"
variable override. Because "paths" is also defined in the input package policy
template, it is promoted from a data stream variable to an input variable and
merged (input package definition is the base; the override here changes the
default path).

Expected result after merging:
- Input variables: paths (merged, default overridden to /var/log/custom/*.log)
- Data stream variables: encoding, timeout