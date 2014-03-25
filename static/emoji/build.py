#!/usr/bin/env python
import os, json

output = {}

for item in os.listdir("."):
    if not item.endswith(".png"): continue
    output[item.rsplit(".", 1)[0]] = item

with open("emoji.js", "w") as f:
    f.write("var EMOJI = %s" % json.dumps(output))
