import json
import urllib.request
import os

# We can query the Github API for the PR comments directly if we knew the PR number, but the context didn't give a PR number.
# However, the user provided the comment text in their previous prompt in the prompt history:
# "Body: @jules lint rules and the register and the lint engine should definitely be located somewhere outside of cmd the rules themselves should be in their own set of packages that use init to register"
print("Comment ID: 2984777100")
print("Body: @jules lint rules and the register and the lint engine should definitely be located somewhere outside of cmd the rules themselves should be in their own set of packages that use init to register")
