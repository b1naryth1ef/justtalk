// Represents the current connection state
var STATE = {
    NIL: 0,
    CONN: 1,
    OK: 2
}

// jt is our global object
var jt = {
    // A dictionary of channels and there data
    channels: {},

    // Handles title-bar based notifications
    title: {
        flash: null,
        origin: null
    },

    // Handles the sent history behavior on up/down arrow in chat box
    history: {
        sent: [],
        point: 0
    },

    // User data
    user: {
        username: "",
        name: "",
        authed: false,
        avatar: ""
    },

    // The users afk info
    afk: {
        timer: null,
        is: false,
    },

    // Users configuration
    config: {
        sound: true,
        notifications: true
    },

    // Handles @ highlights
    highlight: new RegExp(/@([a-zA-Z0-9]+)/g),

    // Handles emojis
    emoji: new RegExp('\:[^\\s:]+\:', 'g'),

    // Websocket
    conn: null,

    // Current state
    state: STATE.NIL,

    // This function is called when the JS is first loaded
    init: function() {
        // If we have a local config stored, reload all the variables
        if (localStorage.getItem("config")) {
            var userconfig = JSON.parse(localStorage.getItem("config"))
            for (varn in userconfig) {
                jt.config[varn] = userconfig[varn]
            }
        }

        // If the user is authed, we open a new websocket, otherwise they
        //  are shown the login modal.
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

        // Set up typeahead on the chat box
        $("#middle-input-text").typeahead({
            source: _.map(_.keys(EMOJI), function (i) { return ':'+i+':'}),
            highlighter: function (item) {
                return jt.to_emoji(item) + item
            },
            triggerChar: ":"
        });
    },

    // Renders the main menu
    renderMenu: function() {
        $(".toggle").remove()
        _.each(jt.config, function (v, k) {
            $(".dropdown-menu").append(TEMPLATES.MENU_ITEM({
                key: k,
                value: v
            }))
        })
    },

    // Send a object over the websocket
    send: function(obj) {
        jt.conn.send(JSON.stringify(obj))
    },

    // Takes a string and replaces emojis with images
    to_emoji: function(s) {
        return s.replace(jt.emoji, function (i) {
            var actual = i.slice(1)
            actual = actual.substring(0, actual.length - 1)
            if (!EMOJI[actual]) { return i}
            return '<span class="emoji" style="background-image:url('+EMOJI[actual]+')"'+actual+'>'+i+'</span>';
        })
    },

    // Display a warning when the socket is closed, and autorefresh
    onSocketClose: function (e) {
        setInterval(function () {
            $.ajax("/api/user", {
                // If we are successful, reload
                success: function () {
                    window.location.reload();
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

    // Creates a fresh websocket
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

    // Flashes a message on the title
    flashTitle: function(text) {
        jt.title.origin = document.title
        clearInterval(jt.title.flash)
        jt.title.flash = setInterval(function() {
            if (window.document.hasFocus()) {
                document.title = jt.title.origin
                clearInterval(jt.title.flash)
                return
            }
            if (document.title == jt.title.origin) {
                document.title = text
            } else {
                document.title = jt.title.origin
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

        jt.history.point = 0
        jt.history.sent.unshift(text)
        if (jt.history.sent.length > 250) {
            jt.history.sent.pop(-1)
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
            if ($(".typeahead").is(":visible")) return;
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

                if (e.which == 38) jt.history.point++
                if (e.which == 40) jt.history.point--

                if (jt.history.point == 250 || jt.history.point > jt.history.sent.length) {
                    jt.history.point = jt.history.sent.length
                    return
                } else if (jt.history.point < 1) {
                    $("#middle-input-text").val("")
                    jt.history.point = 0
                    return
                }
                
                $("#middle-input-text").val(jt.history.sent[jt.history.point-1])
            }
        });

        jt.renderMenu()

        $(".dropdown").delegate(".toggle", "click", function (e) {
            e.stopPropagation();
            var key = $(this).data("key")
            if (jt.config[key] == undefined) return;
            jt.config[key] = !jt.config[key]
            jt.renderMenu()
            localStorage.setItem("config", JSON.stringify(jt.config))
        })

        // Changing channels
        $("#left-box").delegate(".left-box-element", "click", function (e) {
            e.stopPropagation();
            jt.select($(this).data("name"))
            jt.renderUsers()
        })

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
            if (jt.afk.timer) clearTimeout(jt.afk.timer)

            // Un-afk
            if (jt.afk.is) {
                jt.afk.is = false;
                jt.send({
                    "type": "afk",
                    "state": false
                })
            }
        })

        $(window).blur(function () {
            // After 10 minutes set this user as afk
            jt.afk.timer = setTimeout(function () {
                jt.send({
                    "type": "afk",
                    "state": true
                })
                jt.afk.is = true
            }, 60000 * 10)
        })

        $(document).on("drop", function(e) {
            e.preventDefault()
            e.stopPropagation()
            jt.drupload(e.originalEvent.dataTransfer.files);
        })

        $(document).on("dragover", function(e) { e.preventDefault();})
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
            var sel = $('.channel[data-name="'+channel.name+'"]')
            if (channel.selected) {
                sel.show()
                $('.left-chan[data-name="'+channel.name+'"]').addClass("selected")
                $('.left-chan[data-name="'+channel.name+'"]').removeClass("left-box-activity")
                channel.unread = 0
                if (channel.notification) {
                    channel.notification.close()
                }
            } else {
                $('.left-chan[data-name="'+channel.name+'"]').removeClass("selected")
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

            if (window.webkitNotifications.checkPermission() == 0 && jt.config.notifications) {
                channel.notification = new Notification(channel.title, {
                    body: TEMPLATES.NOTIFICATION({
                        username: data.username,
                        msg: data.raw,
                        count: ((channel.unread > 1) ? channel.unread : 0)
                    }),
                    icon: channel.image,
                    tag: channel.name
                })

                // Switch to tab and channel on notification click
                channel.notification.onclick = function(x) {
                    window.focus();
                    if (jt.getCurrentChannel().name != channel.name) {
                        jt.select(channel.name)
                    }
                }
            };
        }

        if (!channel.selected) {
            $("#left-channel-"+chan).addClass("left-box-activity")
        }
    },

    // Adds a channel action
    addAction: function(dest, data) {
        $('.channel[data-name="'+dest+'"]').append(TEMPLATES.CHAT_ACTION(data))
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
            data.msg = data.msg.replace(jt.highlight, "<b>$&</b>")
            data.msg = jt.to_emoji(data.msg)
            content = TEMPLATES.CHAT_CONTENT({
                obj: data,
                time: ""
            })

            if ($('.channel[data-name="'+data.dest+'"] .message-box').last().data("author") == data.username) {
                $('.channel[data-name="'+data.dest+'"] .message-box').last().append(content)
            } else {
                $('.channel[data-name="'+data.dest+'"]').append(TEMPLATES.CHAT_MESSAGE({
                    obj: data,
                    content: content
                }))
            }

            // LOL HACKS
            $("#chat-contents a").attr("target","_blank");
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
                $('.left-chan[data-name="'+data.name+'"]').remove()
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
            $("#chat-contents").append(TEMPLATES.CHAN_DIV({name: data.name}))

            if (data.pm) {
                for (i in data.members) {
                    if (data.members[i].username != jt.user.username) {
                        data.other = data.members[i]
                        break;
                    }
                }

                data.image = data.other.avatar
                data.title = "PM with " + data.other.name
                data.topic = "Private Chat with "+data.other.username
            }

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

            // Save channels
            if (jt.state == STATE.OK) {
                localStorage.setItem("channels", JSON.stringify(_.keys(jt.channels)))
            }

            $('channel[data-name="'+data.name+'"]').remove()
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