import json, sys, os, redis, requests

CONFIG = {
    "plugins": []
}

PLUGINS = {}

r = redis.Redis()

class API(object):
    def __init__(self, url="http://localhost:5000"):
        self.url = url

    def request(self, route, data):
        r = requests.post(self.url+"/api/"+route, data=json.dumps(data))
        r.raise_for_status()
        return r

    def send_action(self, channel, text, icon="", user=""):
        data = {
            "channel": channel,
            "msg": text,
            "icon": icon,
        }
        if user:
            data['user'] = user
        return self.request("send", data)

class Handler():
    def __init__(self):
        self.commands = {}

    def bind(self, cmd, func):
        self.commands[cmd] = func

HANDLE = Handler()

def save_config():
    print "Saving Config"
    with open("config.js", "w") as f:
        json.dump(config, f)

def load_config():
    global CONFIG
    print "Loading Config"
    if not os.path.exists("config.js"):
        save_config()

    with open("config.js", "r") as f:
        CONFIG = json.load(f)

def load_plugins():
    print "Loading Plugins"
    for plugin in CONFIG.get("plugins", []):
        print "Loading plugin %s" % plugin
        __import__(plugin)
        PLUGINS[plugin] = sys.modules[plugin]
        print PLUGINS[plugin]
        if hasattr(PLUGINS[plugin], "init"):
            PLUGINS[plugin].init(CONFIG.get(plugin, {}), HANDLE)

def loop():
    sub = r.pubsub()
    sub.psubscribe("justtalk-*")
    for msg in sub.listen():
        if msg['type'] == "pmessage":
            data = json.loads(msg['data'])
            if data['msg'].startswith("!"):
                cmd = data['msg'].split(" ", 1)[0][1:]
                data['args'] = data['msg'].split(" ")[1:]
                if cmd in HANDLE.commands.keys():
                    HANDLE.commands[cmd](data)
            else:
                for plugin in PLUGINS.values():
                    if hasattr(plugin, "handle"):
                        plugin.handle(data)

if __name__ == '__main__':
    load_config()
    load_plugins()
    loop()
