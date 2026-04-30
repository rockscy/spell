# spell demo — recorded with vhs (https://github.com/charmbracelet/vhs)
#
# Reproducing this demo:
#   1. brew install vhs ffmpeg ttyd
#   2. spell init  (configure any provider)
#   3. cd into a directory with a few sample files of varying sizes
#   4. SPELL_CONFIG=$(spell --where) vhs docs/demo.tape
#
# The Env line below points at a sandbox config so this script does not
# accidentally use the maintainer's real ~/.config/spell/config.toml.
# Override SPELL_CONFIG (or remove the Env line) if you record it yourself.

Output docs/demo.gif

Set Width      1100
Set Height     620
Set FontSize   18
Set FontFamily "Menlo"
Set Padding    24
Set Theme      "Catppuccin Mocha"

Env SPELL_CONFIG "/tmp/spell-demo-config/config.toml"

# ---- silent setup ----
Hide
Type "cd /tmp/spell-demo-stage"
Enter
Type "clear"
Enter
Show

# ---- show the directory so the answer is meaningful ----
Type "ls -lh"
Sleep 400ms
Enter
Sleep 1500ms

# ---- the actual demo: one-shot natural language query ----
Type "spell list the 3 biggest files in this directory"
Sleep 600ms
Enter

# Wait for: spinner → command lands in the input box.
# mimo-v2.5 typically resolves in ~3 seconds; 6s leaves headroom.
Sleep 6s

# User confirms — command executes inline, output appears below.
Enter
Sleep 2500ms
