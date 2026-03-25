import json
import sys

# We don't have direct access to the tool output, but I can ask the tool again and pipe it.
# Actually, I can't pipe tool output in bash directly, the tool is a python function in the environment.
# Wait, the user already provided the comment in the previous prompt!
# Let me re-read the prompt history carefully.
