import requests, json, thread, time, re, random
from manager import API

CONFIG = None
api = API()

DAY_TYPE = ['good', 'swaggy', 'great', 'awesome', 'amazing', 'perfect']
LUNCH_TYPE = ['delicious', 'om nom nomy', 'yummy', 'tasty', 'yoloy']


def lunch(obj):
    print api.r.lrange("plugin-lunch", 0, -1)
    msg = "It's a %s day for some %s %s!" % (
        random.choice(DAY_TYPE),
        random.choice(LUNCH_TYPE),
        random.choice(api.r.lrange("plugin-lunch", 0, -1)))
    api.send_action(obj['dest'], msg, icon="coffee")

def addlunch(obj):
    _, place = obj['raw'].split(" ", 1)
    api.r.lpush("plugin-lunch", place.strip())
    api.send_action(obj['dest'], "Added %s to the lunch list!" % place.strip(), icon="thumbs-up")

def init(config, handle):
    global CONFIG
    CONFIG = config
    handle.bind("lunch", lunch)
    handle.bind("addlunch", addlunch)
