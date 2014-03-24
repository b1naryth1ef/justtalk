import requests, json, thread, time, re, bleach
from manager import API

HREF = re.compile(r"""href=[\'"]?([^\'" >]+)""")
TICKET = re.compile(r'#([0-9]+)')
HASH = re.compile(r'[0-9a-f]{5,40}')

CONFIG = None
CACHED_REPOS = None
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
        return self.request("%s/tickets" % CONFIG.get("repo"), query="id:%s" % id).json()

    def get_activity(self):
        return map(lambda i: i.get("event"), self.request("activity").json())

    def get_repos(self):
        return map(lambda i: i.get("repository"), self.request("/%s/repositories" % CONFIG.get("repo")).json())

    def get_commit(self, repo, ref):
        return self.request("/%s/%s/commits/%s" % (CONFIG.get("repo"), repo, ref)).json()

cb = None

def format_ticket(dest, id):
    data = cb.get_ticket(id)[-1]['ticket']
    url = '<a target="_blank" href="https://%s.codebasehq.com/projects/%s/tickets/%s">#%s</a>' % (
        CONFIG.get("account"), CONFIG.get("repo"), id, id)
    msg = "Ticket %s: %s (%s)" % (url, bleach.clean(data["summary"]),
        bleach.clean(data["status"]['name']))
    api.send_action(dest, msg, icon="ticket")

def format_commit(dest, data, repo):
    url = "https://spoton.codebasehq.com/projects/%s/repositories/%s/commit/%s" % (
        CONFIG.get("repo"), repo, data['ref'])
    link = '<a target="_blank" href="%s">%s</a>' % (url, data['ref'][:10])
    msg = "Commit %s: %s (%s)" % (link, bleach.clean(data['message']), data['author_email'])
    icon = "code-fork" if "merge branch" in data['message'].lower() else "code"
    api.send_action(dest, msg, icon=icon)

def ticket(obj):
    if len(obj['args']) < 1:
        api.send_action(obj['dest'], "Usage: !ticket <ticket num>")
        return
    try:
        format_ticket(obj['dest'], obj['args'][0])
    except Exception as e:
        print e
        api.send_action(obj['dest'], "Woahh... Something went wrong while processing your request!")

def activity_loop():
    LAST = None
    while True:
        data = cb.get_activity()
        for event in data:
            if event['id'] <= LAST: continue
            data = HREF.findall(event['html_text'])
            if not len(data): continue
            url = ("https://%s.codebasehq.com" % CONFIG.get("account"))+data[0]
            msg = '<a target="_blank" href="%s">%s</a>' % (url, event['title'])

            if event['type'] == "push":
                icon = "upload"
            else:
                icon = ""
                print event['type']

            api.send_action("devs", msg, icon=icon)
            LAST = event['id']
        time.sleep(30)

def init(config, handle):
    global CONFIG, cb, CACHED_REPOS
    CONFIG = config
    handle.bind("ticket", ticket)

    cb = Codebase(CONFIG.get("username"), CONFIG.get("password"))
    CACHED_REPOS = cb.get_repos()

    thread.start_new_thread(activity_loop, ())

def try_get_commit(zhash, data, repo):
    try:
        com = cb.get_commit(repo['permalink'], zhash)
    except: return
    if not len(com): return
    format_commit(data['dest'], com[0]['commit'], repo['permalink'])


def handle(data):
    has_any_ticket = TICKET.findall(data['raw'])
    for item in has_any_ticket:
        if len(item) >= 4:
            format_ticket(data['dest'], item)

    has_any_hash = HASH.findall(data['raw'])
    for zhash in has_any_hash:
        for repo in CACHED_REPOS:
            thread.start_new_thread(try_get_commit, (zhash, data, repo))
