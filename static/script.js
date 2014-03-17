function isNumber(n) {
  return !isNaN(parseFloat(n)) && isFinite(n);
}

view_main = {
    channels: {},
    timer: null,
    notifications: false,

    flashTitle: function(text) {
        if (view_main.timer) { return }

        var original = document.title

        $(window).blur(function () {
            view_main.timer = setInterval(function() {
                var val = $('title').text() == original ? text : original;
                $('title').text(val);
            }, 1000)
        })

        $(window).focus(function() {
            clearInterval(view_main.timer)
            view_main.timer = null;
            document.title = original;
        })
    },

    // Gets the currently active channel
    getCurrentChannel: function () {
        for (i in view_main.channels) {
            if (view_main.channels[i].selected) {
                return view_main.channels[i]
            }
        }
    },

    // Called when a user sends a message
    onSendMessage: function() {
        jt.send({
            "type": "msg",
            "msg": $("#middle-input-text").val(),
            "dest": view_main.getCurrentChannel().name
        })
        $("#middle-input-text").val("")
    },

    // First time render, should only be called once ideally
    render: function() {
        // Input
        $('#middle-input-text').keypress(function(e) {
            if(e.which == 13) {
                view_main.onSendMessage();
                e.preventDefault();
            }
        });

        // Changing channels
        $("#left-box").delegate(".left-box-element", "click", function (e) {
            view_main.select($(this).data("name"))
            view_main.renderUsers()
        })

        if (window.webkitNotifications.checkPermission() == 0) {
            $("#toggle-notifications").hide()
        }

        $("#toggle-notifications").click(function (e) {
            window.webkitNotifications.requestPermission(function (a, b) {
                console.log(a)
                console.log(b)
            });
        })

        $("html").on("drop", function(event) {
            event.preventDefault();  
            event.stopPropagation();
            alert("Dropped!");
            console.log("DROPPED");
        });

    },

    // Renders the left hand channel list
    renderChannels: function () {
        $("#left-box").empty()
        _.each(view_main.channels, function(v, i) {
            console.log(v)
            $("#left-box").append(TEMPLATES.CHANNEL_LEFT({
                obj: v
            }))
        })
    },

    // Renders the right hand user list
    renderUsers: function() {
        $("#users-online-now").empty()
        var total = 0
        _.each(view_main.getCurrentChannel().members, function(user, y) {
            $("#users-online-now").append(TEMPLATES.USER_RIGHT({
                obj: user
            }))
            total += 1
        })
        $("#online-count").text(total)
    },

    // Changes the selected channel
    select: function(chan) {
        _.each(view_main.channels, function(channel, x) {
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
        var channel = view_main.channels[chan];
        if (channel.selected) {
            return;
        }

        // Increment unread counter
        channel.unread += 1


        if (data) {
            if (window.webkitNotifications.checkPermission() == 0) {
                channel.notification = new Notification(channel.title, {
                    body: TEMPLATES.NOTIFICATION({
                        username: data.username,
                        msg: data.msg,
                        count: channel.unread - 1
                    }),
                    icon: channel.image,
                    tag: channel.name
                })
            };
        }

        // view_main.flashTitle("Actvity in "+view_main.channels[chan].title)
        $("#left-channel-"+chan).addClass("left-box-activity")
    },

    // Adds a channel action
    addAction: function(dest, data) {
        $("#channel-"+dest).append(TEMPLATES.CHAT_ACTION(data))
        view_main.pingChannel(dest)
    },

    // Handles incoming websocket data
    handle: function(data) {
        if (data.type == "msg") {
            $("#channel-"+data.dest).append(TEMPLATES.CHAT_MESSAGE({
                obj: data,
                time: "12 seconds ago"
            }))
            view_main.pingChannel(data.dest, data)
        }

        if (data.type == "action") {
            view_main.addAction(data.dest, {
                obj: data,
                color: null
            })
        }

        if (data.type == "quit") {
            if (!view_main.channels[data.name]) {return}
            if (data.user == jt.user.username) {return}
            for (i in view_main.channels[data.name].members) {
                if (view_main.channels[data.name].members[i].username == data.user) {
                    view_main.channels[data.name].members.splice(i, 1)
                    view_main.renderChannels()
                    view_main.renderUsers()
                    view_main.addAction(data.name, {
                        obj: {
                            action: data.user + " has left the channel",
                            icon: "sign-out"
                        },
                        color: null
                    })
                    return
                }
            }
        }

        if (data.type == "join") {
            if (!view_main.channels[data.name]) {return}
            if (data.user == jt.user.username) {return}
            for (i in view_main.channels[data.name].members) {
                if (view_main.channels[data.name].members[i].username == data.user.username) {
                    return
                }
            }
            view_main.channels[data.name].members.push(data.user)
            view_main.renderChannels()
            view_main.renderUsers()
            view_main.addAction(data.name, {
                obj: {
                    action: data.user.username + " has joined the channel",
                    icon: "sign-in"
                },
                color: null
            })

        }

        if (data.type == "error") {
            view_main.addAction(view_main.getCurrentChannel().name, {
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
            view_main.channels[data.name] = data
            view_main.select(data.name)
            view_main.renderChannels()
            view_main.renderUsers()
            localStorage.setItem("channels", JSON.stringify(_.keys(view_main.channels)))
        }

        if (data.type == "updatechannel") {
            view_main.channels[data.name][data.k] = data.v
            view_main.renderChannels()
            view_main.addAction(data.name, {
                obj: {
                    action: data.a,
                    icon: "pencil"
                },
                color: null
            })
        }

        if (data.type == "channelclose") {
            delete view_main.channels[data.name]

            if (view_main.channels) {
                for (chan in view_main.channels) {
                    view_main.select(chan)
                    break;
                }
            }

            $("#channel-"+data.name).remove()
            view_main.renderChannels()
            view_main.renderUsers()
        }
    }
}

// No, not justin... never justin...
jt = {
    conn: null,
    user: {
        username: "andrei@spoton.com",
        name: "Andrei",
        authed: false,
        avatar: ""
    },
    view: view_main,

    init: function() {
        if (localStorage.getItem("username")) {
            jt.setupWebSocket(
                localStorage.getItem("username"),
                localStorage.getItem("password")
            );
        } else {
            $("#login").modal("show")
            $("#login-button").click(function (e) {
                localStorage.setItem("username", $("#login-username").val());
                localStorage.setItem("password", "");
                jt.setupWebSocket(
                    $("#login-username").val(),
                    ""//$("#login-password").val()
                );
            })
        }
        jt.view.render()
    },

    send: function(obj) {
        jt.conn.send(JSON.stringify(obj))
    },

    onSocketClose: function (e) {
        //$(".container-fluid").addClass("body-error")
        $(".overlay").show()
        $("#navbar").hide();
        $("#conn-lost").show();
        // alert("Websocket closed!");
    },


    onSocketMessage: function (e) {
        var obj = JSON.parse(e.data);
        console.log(obj)
        switch (obj.type) {
            case "hello":
                if (obj.success) {
                    if (localStorage.getItem("channels")) {
                        jt.send({
                            "type": "join",
                            "channels": JSON.parse(localStorage.getItem("channels"))
                        })
                    }
                    jt.user.authed = true;
                    jt.user.avatar = obj.avatar;
                    $("#chat-image").attr("src", jt.user.avatar)
                    $("#login").modal("hide")
                } else {
                    alert("Could not login!");
                }
                break;
        }

        jt.view.handle(obj)
    },

    setupWebSocket: function(username, password) {
        if (window["WebSocket"]) {
            var port = window.location.port ? "" : "5000"
            jt.conn = new WebSocket("ws://"+window.location.host+":"+port+"/socket");
            jt.conn.onclose = jt.onSocketClose;
            jt.conn.onmessage = jt.onSocketMessage;
            jt.conn.onopen = function () {
                jt.send({
                    "type": "hello",
                    "username": username,
                    "name": username.split("@")[0],
                    "password": password // HASH lewl
                })
            }
        } else {
            alert("Your browser does not have websocket support :(");
        }
    }
}

$(document).ready(function () {
    jt.init()
});