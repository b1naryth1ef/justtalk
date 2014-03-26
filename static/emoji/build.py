#!/usr/bin/env python
import os, json, requests, subprocess

output = {}

def run(cmd):
    return subprocess.Popen(cmd, shell=True).wait()

HIPCHAT_SOURCE = "https://raw.githubusercontent.com/henrik/hipchat-emoticons/master/emoticons.json"
def parse_hipchat_source():
    data = requests.get(HIPCHAT_SOURCE).json()
    data = set(map(lambda i: i['file'], data))

    for item in data:
        yield (item.rsplit(".", 1)[0], "https://dujrsrsgsd3nh.cloudfront.net/img/emoticons/"+item)

def parse_emoji_source():
    if os.path.exists("/tmp/emoji"):
        run("cd /tmp/emoji; git reset --hard; git pull")
    else:
        run("git clone https://github.com/arvida/emoji-cheat-sheet.com.git /tmp/emoji")
    items = [i for i in os.listdir("/tmp/emoji/public/graphics/emojis/") if i.endswith(".png")]

    for item in items:
        yield (item.rsplit(".", 1)[0], "https://assets.github.com/images/icons/emoji/"+item)

def build():
    print "Building full emoticon list from all sources"
    for key, value in list(parse_hipchat_source())+list(parse_emoji_source()):
        output[key] = value

    print "Saving emoticon list as json"
    with open("emoji.js", "w") as f:
        f.write("var EMOJI = %s" % json.dumps(output))
    print "DONE!"

if __name__ == '__main__':
    build()
