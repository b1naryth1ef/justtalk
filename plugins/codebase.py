import requests, json, thread, time, re
from manager import API

HREF = re.compile("""href=[\'"]?([^\'" >]+)""")

CONFIG = None
api = API()

class Codebase():
    def __init__(self, user, passwd):
        self.username = user
        self.password = passwd

    def request(self, endpoint, **data):
        r = requests.get("https://api3.codebasehq.com/"+endpoint, params=data,
            auth=(self.username, self.password), headers={
                "accept": "application/json",
                "content-type": "application/json"
            })
        r.raise_for_status()
        return r

    def get_ticket(self, id):
        return self.request("backend/tickets", query="id:%s" % id).json()

    def get_activity(self):
        return map(lambda i: i.get("event"), self.request("activity").json())

    def get_repos(self):
        return map(lambda i: i.get("repository"), self.request("/%s/repositories").json() % CONIFG.get("repo"))

cb = None

def ticket(obj):
    global CONFIG
    if len(obj['args']) < 1:
        api.send_action(obj['channel'], "Usage: !ticket <ticket num>")
        return
    try:
        data = cb.get_ticket(obj['args'][0])[-1]['ticket']
        url = '<a href="https://%s.codebasehq.com/projects/%s/tickets/%s">#%s</a>' % (
            CONFIG.get("account"), CONIFG.get("repo"), obj['args'][0], obj['args'][0])
        msg = "Ticket %s: %s (%s)" % (url, data["summary"], data["status"]['name'])
        api.send_action(obj['channel'], msg)
    except:
        api.send_action(obj['channel'], "Woahh... Something went wrong while processing your request!")

def activity_loop():
    LAST = None
    while True:
        data = cb.get_activity()
        for event in data:
            print LAST, event['id']
            if event['id'] <= LAST: continue
            url = ("https://%s.codebasehq.com" % CONIFG.get("account"))+HREF.findall(event['html_text'])[0]
            msg = '<a href="%s">%s</a>' % (url, event['title'])

            if event['type'] == "push":
                icon = "upload"
            else:
                icon = ""
                print event['type']

            api.send_action("devs", msg, icon=icon)
            LAST = event['id']
        time.sleep(30)

def init(config, handle):
    global CONFIG, cb
    CONFIG = config
    handle.bind("ticket", ticket)

    cb = Codebase(CONFIG.get("username"), CONFIG.get("password"))

    thread.start_new_thread(activity_loop, ())

def handle(data):
    pass

