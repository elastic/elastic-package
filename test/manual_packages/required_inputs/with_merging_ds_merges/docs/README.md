# Variable Merging - Data Stream Merges

Test fixture: composable package with no policy template variable overrides.
The data stream manifest overrides the "encoding" variable from the input
package (providing a different title) and adds a new "custom_tag" variable.

Expected result after merging:
- Input variables: (none)
- Data stream variables:
  - paths (unchanged from input package)
  - encoding (merged: base from input pkg, title overridden)
  - timeout (unchanged from input package)
  - custom_tag (new, from data stream manifest)