Rules

Never write em-dashes, in code, comments or docs.
Comments explain why, never what. Do not comment obvious code, do not split logic or struct fields with comment lines, and do not write long comment sections. Short trailing notes on a field are fine.
Build bottom up: the model a step needs, then the service that uses it, then the next model. Not all models first, not a service before its model.
Dependencies are injected. More than three go in a Deps struct. No package level state, no init() that connects to anything.
Errors wrap with %w, start lowercase, are handled or returned, not both. There is no HTTP layer; module sentinel errors are mapped to codes at the editor boundary (internal/editor/service).
ctx context.Context is the first parameter of anything that does I/O.
Tests use the standard library, sit next to their subject, run under -race, are named TestUnit_Behaviour and use Arrange / Act / Assert comments.
Add a dependency in the same commit that uses it. Standard library first.
Embedded files (frontend/dist, templates) and the linker-set game data key (bundle.pakSecret) are the only package level values; they are read-only build artefacts.
The game data key comes from .env (ASCDII_PAK_SECRET, git ignored, written by the first make); never put a real key in the source.
