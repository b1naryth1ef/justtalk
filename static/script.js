function isNumber(n) {
  return !isNaN(parseFloat(n)) && isFinite(n);
}

(function($) {
    $.fn.getCursorPosition = function() {
        var input = this.get(0);
        if (!input) return; // No (input) element found
        if ('selectionStart' in input) {
            // Standard-compliant browsers
            return input.selectionStart;
        } else if (document.selection) {
            // IE
            input.focus();
            var sel = document.selection.createRange();
            var selLen = document.selection.createRange().text.length;
            sel.moveStart('character', -input.value.length);
            return sel.text.length - selLen;
        }
    }
})(jQuery);


var STATE = {
    NIL: 0,
    CONN: 1,
    OK: 2
}

var jt = {
    channels: {},
    title_flash: null,
    title_origin: null,
    sent_history: [],
    history_point: 0,
    highlight: new RegExp(/@([a-zA-Z0-9]+)/g),
    conn: null,
    user: {
        username: "",
        name: "",
        authed: false,
        avatar: ""
    },
    view: jt,
    afk_timer: null,
    is_afk: false,
    state: STATE.NIL,

    config: {
        sound: true,
    },


    // If the user is authed, we open a new websocket, otherwise they
    //  are shown the login modal.
    init: function() {
        emoji.img_path = "https://raw.githubusercontent.com/github/gemoji/master/images/emoji/unicode/";
        if (localStorage.getItem("config")) {
            jt.config = JSON.parse(localStorage.getItem("config"))
        }

        $.ajax("/api/user", {
            success: function(data) {
                if (!data.success) {
                    $("#login").modal("show")
                    $("#login-button").click(function (e) {
                        window.location = "/authorize"
                    })
                } else {
                    jt.setupWebSocket()
                }
            }
        })

        // Render everything else
        jt.render()
    },

    // Send a object over the websocket
    send: function(obj) {
        jt.conn.send(JSON.stringify(obj))
    },

    // Display a warning when the socket is closed, and autorefresh
    onSocketClose: function (e) {
        setInterval(function () {
            $.ajax("/api/user", {
                // If we are successful, reload
                success: function () {
                    window.location = "/"
                }
            })
        }, 4000)
        $(".overlay").show()
        $("#navbar").hide();
        $("#conn-lost").show();
    },

    // Handle messages
    onSocketMessage: function (e) {
        var obj = JSON.parse(e.data);
        switch (obj.type) {
            case "hello":
                if (obj.success) {
                    jt.state = STATE.OK
                    if (localStorage.getItem("channels")) {
                        jt.send({
                            "type": "join",
                            "channels": JSON.parse(localStorage.getItem("channels"))
                        })
                    }
                    jt.user = obj.user
                    jt.user.authed = true;
                    $("#chat-image").attr("src", jt.user.avatar)
                    $("#login").modal("hide")
                } else {
                    alert("Could not login: "+obj.msg);
                }
                break;
        }

        jt.handle(obj)
    },

    setupWebSocket: function() {
        jt.state = STATE.CONN
        if (window["WebSocket"]) {
            var port = window.location.port ? "" : "5000"
            jt.conn = new WebSocket("ws://"+window.location.host+":"+port+"/socket");
            jt.conn.onclose = jt.onSocketClose;
            jt.conn.onmessage = jt.onSocketMessage;
            jt.conn.onopen = function () {
                jt.send({"type": "hello"})
            }
        } else {
            alert("Your browser does not have websocket support :(");
        }
    },

    flashTitle: function(text) {
        jt.title_origin = document.title
        clearInterval(jt.title_flash)
        jt.title_flash = setInterval(function() {
            if (window.document.hasFocus()) {
                document.title = jt.title_origin
                clearInterval(jt.title_flash)
                return
            }
            if (document.title == jt.title_origin) {
                document.title = text
            } else {
                document.title = jt.title_origin
            }
        }, 1000);
    },

    // Gets the currently active channel
    getCurrentChannel: function () {
        for (i in jt.channels) {
            if (jt.channels[i].selected) {
                return jt.channels[i]
            }
        }
    },

    // Called when a user sends a message
    onSendMessage: function() {
        var text = $("#middle-input-text").val()

        jt.history_point = 0
        jt.sent_history.unshift(text)
        if (jt.sent_history.length > 250) {
            jt.sent_history.pop(-1)
        }

        jt.send({
            "type": "msg",
            "msg": text,
            "dest": jt.getCurrentChannel().name
        })
        $("#middle-input-text").val("")
    },

    // First time render, should only be called once ideally
    render: function() {
        // Input
        $('#middle-input-text').keydown(function(e) {
            // Enter key
            if (e.which == 13) {
                jt.onSendMessage();
            }

            // Tab autocomplete
            if (e.which == 9) {
                var x = $("#middle-input-text").getCursorPosition(),
                    text = $("#middle-input-text").val()
                    data = ""

                // Extract the token, e.g. "@te" >> "te"
                while (x > 0) {
                    x--

                    // If we have a token
                    if (text[x] == "@") {
                        var results = []
                        _.each(jt.getCurrentChannel().members, function (v, k) {
                            if (v.username.toLowerCase().indexOf(data) == 0 ||
                                    v.name.toLowerCase().indexOf(data) == 0) {
                                results.push(v)
                            }
                        })

                        // If we have exactly one match, build a new input value based on it
                        if (results.length == 1) {
                            var a = text.substr(0, x)
                            var b = text.substr(x + data.length + 1, 1000)
                            $("#middle-input-text").val(a + "@" + results[0].name.toLowerCase() +" " + b)
                        }
                        break;
                    }

                    // Break on space, we don't do those
                    if (text[x] == " ") {
                        break;
                    }

                    // Append char
                    data = $("#middle-input-text").val()[x].toLowerCase() + data
                }

                e.preventDefault();
            }

            // Handles up/down arrow history
            if (e.which == 38 || e.which == 40) { 
                e.preventDefault();

                if (e.which == 38) jt.history_point++
                if (e.which == 40) jt.history_point--

                if (jt.history_point == 250 || jt.history_point > jt.sent_history.length) {
                    jt.history_point = jt.sent_history.length
                    return
                } else if (jt.history_point < 1) {
                    $("#middle-input-text").val("")
                    jt.history_point = 0
                    return
                }
                
                $("#middle-input-text").val(jt.sent_history[jt.history_point-1])
            }
        });

        // Changing channels
        $("#left-box").delegate(".left-box-element", "click", function (e) {
            jt.select($(this).data("name"))
            jt.renderUsers()
        })

        if (window.webkitNotifications.checkPermission() == 0) {
            $("#toggle-notifications").hide()
        }

        $("#toggle-notifications").click(function (e) {
            window.webkitNotifications.requestPermission();
        })

        // Clears notifications on window focus gain
        $(window).focus(function () {
            _.each(jt.channels, function (v) {
                if (v.selected) {
                    if (v.notification) v.notification.close();
                    v.unread = 0
                }
            })

            // Clear afk timer
            if (jt.afk_timer) clearTimeout(jt.afk_timer)

            // Un-afk
            if (jt.is_afk) {
                jt.is_afk = false;
                jt.send({
                    "type": "afk",
                    "state": false
                })
            }
        })

        $(window).blur(function () {
            // After 10 minutes set this user as afk
            jt.afk_timer = setTimeout(function () {
                jt.send({
                    "type": "afk",
                    "state": true
                })
                jt.is_afk = true
            }, 60000 * 10)
        })

        $(document).on("drop", function(e) {
            e.preventDefault()
            e.stopPropagation()
            jt.drupload(e.originalEvent.dataTransfer.files);
        })

        $(document).on("dragover", function(e) { e.preventDefault(); console.log("ding")})
    },

    // Renders the left hand channel list
    renderChannels: function () {
        $("#left-box").empty()
        _.each(jt.channels, function(v, i) {
            $("#left-box").append(TEMPLATES.CHANNEL_LEFT({
                obj: v
            }))
        })
    },

    // Renders the right hand user list
    renderUsers: function() {
        $("#users-online-now").empty()
        _.each(jt.getCurrentChannel().members, function(user, y) {
            $("#users-online-now").append(TEMPLATES.USER_RIGHT({
                obj: user
            }))
        })
    },

    // Changes the selected channel
    select: function(chan) {
        _.each(jt.channels, function(channel, x) {
            channel.selected = (x == chan)
            var sel = $("#channel-"+channel.name)

            if (channel.selected) {
                sel.show()
                $("#left-channel-"+channel.name).addClass("selected")
                $("#left-channel-"+channel.name).removeClass("left-box-activity")
                channel.unread = 0
                if (channel.notification) {
                    channel.notification.close()
                }
            } else {
                $("#left-channel-"+channel.name).removeClass("selected")
                sel.hide()
            }
        })
        $("#chat-contents").scrollTop($("#chat-contents")[0].scrollHeight);
    },

    // Marks a background channel when it has "changed" (e.g. chat/action)
    //  in some way.
    pingChannel: function(chan, data) {
        $("#chat-contents").scrollTop($("#chat-contents")[0].scrollHeight);
        var channel = jt.channels[chan];

        if (data && !window.document.hasFocus()) {
            if (data.msg) {
                channel.unread += 1
            }
            jt.flashTitle("Actvity in "+jt.channels[chan].title)

            if (window.webkitNotifications.checkPermission() == 0) {
                channel.notification = new Notification(channel.title, {
                    body: TEMPLATES.NOTIFICATION({
                        username: data.username,
                        msg: data.raw,
                        count: ((channel.unread > 1) ? channel.unread : 0)
                    }),
                    icon: channel.image,
                    tag: channel.name
                })
            };
        }

        if (!channel.selected) {
            $("#left-channel-"+chan).addClass("left-box-activity")
        }
    },

    // Adds a channel action
    addAction: function(dest, data) {
        $("#channel-"+dest).append(TEMPLATES.CHAT_ACTION(data))
        jt.pingChannel(dest)
    },

    // Handles incoming websocket data
    handle: function(data) {
        if (data.type == "msg") {
            data.highlight = false

            // Highlight @ mentions
            if (data.username != jt.user.username) {
                var hilights = data.msg.match(jt.highlight)
                if (hilights) {
                    for (i in hilights) {
                        // Its important to us!
                        if (hilights[i].toLowerCase() == "@all" || hilights[i].toLowerCase() == "@"+jt.user.name.toLowerCase()) {
                            data.highlight = true
                            if (jt.config.sound) new Audio("ding.mp3").play()
                        }
                    }
                }
            }

            // Bold all highlights
            data.msg = emoji.replace_colons(data.msg.replace(jt.highlight, "<b>$&</b>"))
            content = TEMPLATES.CHAT_CONTENT({
                obj: data,
                time: ""
            })

            if (false) { //($("#channel-"+data.dest+" .message-box").last().data("author") == data.username) {
                $("#channel-"+data.dest+" .message-box").last().append(content)
            } else {
                $("#channel-"+data.dest).append(TEMPLATES.CHAT_MESSAGE({
                    obj: data,
                    content: content
                }))
            }
            jt.pingChannel(data.dest, data)
        }

        if (data.type == "afk") {
            _.each(jt.channels, function(v, k) {
                _.each(v.members, function(y, x) {
                    if (y.username == data.user) {
                        y.afk = data.state
                    }
                })
            })
            jt.renderUsers()
        }

        if (data.type == "action") {
            jt.addAction(data.dest, {
                obj: data,
                color: data.color || null
            })
        }

        if (data.type == "quit") {
            if (!jt.channels[data.name]) {return}
            // todo functionaize
            for (i in jt.channels[data.name].members) {
                if (jt.channels[data.name].members[i].username == data.user) {
                    jt.channels[data.name].members.splice(i, 1)
                    jt.renderChannels()
                    jt.renderUsers()
                    jt.addAction(data.name, {
                        obj: {
                            action: data.user + " has left the channel",
                            icon: "sign-out"
                        },
                        color: null
                    })
                    break
                }
            }

            if (data.user == jt.user.username) {
                delete jt.channels[data.name]

                if (jt.channels) {
                    for (chan in jt.channels) {
                        jt.select(chan)
                        break;
                    }
                }
                $("#channel-"+data.name).remove()
                jt.renderChannels()
                jt.renderUsers()
            }
        }

        if (data.type == "join") {
            if (!jt.channels[data.name]) {return}
            if (data.user == jt.user.username) {return}
            for (i in jt.channels[data.name].members) {
                if (jt.channels[data.name].members[i].username == data.user.username) {
                    return
                }
            }
            jt.channels[data.name].members.push(data.user)
            jt.renderChannels()
            jt.renderUsers()
            jt.addAction(data.name, {
                obj: {
                    action: data.user.username + " has joined the channel",
                    icon: "sign-in"
                },
                color: null
            })

        }

        if (data.type == "error") {
            jt.addAction(jt.getCurrentChannel().name, {
                obj: {
                    action: data.msg,
                    icon: "warning"
                },
                color: "#B80000"
            })
        }

        if (data.type == "channel") {
            $("#chat-contents").append('<div style="display: none" id="channel-'+data.name+'"></div>')
            data.unread = 0
            jt.channels[data.name] = data
            jt.select(data.name)
            jt.renderChannels()
            jt.renderUsers()

            // Only set this post-connect or you'll screw with channel rejoins
            if (jt.state == STATE.OK) {
                localStorage.setItem("channels", JSON.stringify(_.keys(jt.channels)))
            }
        }

        if (data.type == "updatechannel") {
            jt.channels[data.name][data.k] = data.v
            jt.renderChannels()
            jt.addAction(data.name, {
                obj: {
                    action: data.a,
                    icon: "pencil"
                },
                color: null
            })
        }

        if (data.type == "channelclose") {
            delete jt.channels[data.name]

            if (jt.channels) {
                for (chan in jt.channels) {
                    jt.select(chan)
                    break;
                }
            }

            $("#channel-"+data.name).remove()
            jt.renderChannels()
            jt.renderUsers()
        }
    },

    drupload: function(files) {
        var formData = new FormData();
        for (var i = 0; i < files.length; i++) {
            formData.append('file', files[i]);
        }

        var xhr = new XMLHttpRequest();
        xhr.open('POST', '/api/upload?channel='+jt.getCurrentChannel().name);
        xhr.onload = function () {
            if (xhr.status != 200) {
                alert("Uh Oh! Something went wonkers!");
            }
        };
        xhr.send(formData);
        }
}

$(document).ready(function () {
    jt.init()
});