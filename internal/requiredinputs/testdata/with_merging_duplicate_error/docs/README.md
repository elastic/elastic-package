# Variable Merging - Duplicate Error

Test fixture: composable package whose data stream manifest defines the "paths"
variable twice. The merging algorithm must detect this duplicate and return an
error (Step 5: fail if there are multiple variables with the same name).

Expected result: error indicating a duplicate variable name "paths" in the data
stream variable list.