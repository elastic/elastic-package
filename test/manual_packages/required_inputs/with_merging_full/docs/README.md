# Variable Merging - Full Mix

Test fixture: composable package that exercises all five variable merging steps
from SPEC.md simultaneously.

Policy template input vars (Step 2 → Step 3 promotion):
- "paths" override with new default → promoted to input variable
- "encoding" override with show_user:true → promoted to input variable

Data stream manifest vars (Step 4 merge):
- "timeout" override with new description → merged with remaining DS variable
- "custom_tag" new variable → added to DS variables

Expected result after merging:
- Input variables:
  - paths (merged: base from input pkg, default overridden to /var/log/custom/*.log)
  - encoding (merged: base from input pkg, show_user overridden to true)
- Data stream variables:
  - timeout (merged: base from input pkg, description overridden)
  - custom_tag (new, from data stream manifest)