
CHAT_MESSAGE = _.template(
    '<div class="message-box">'+
    '<img src="<%= obj.avatar %>" class="img-rounded chat-avatar"><b><%= obj.username %></b><br />'+
    '<span><%= obj.msg %></span></div>')

CHAT_ACTION = _.template(
'<div class="message-box message-box-action">'+
    '<% if (obj.icon) { %><i class="glyphicon glyphicon-<%= obj.icon %>"></i><% } %>'+
    '<i style="margin-left: 10px; <% if (color) {%>color: <%= color %> <% } %>"><%= obj.action %></i></div>')

CHANNEL_LEFT = _.template(
'<div id="left-channel-<%= obj.name %>" class="left-box-element <% if (obj.selected) { %>selected<% } %>" data-name="<%= obj.name %>">'+
'<img src="<%= obj.image %>" class="img-rounded" style="float: left; margin-right: 5px; height: 40px;">'+
'<h4 style="margin-top: 0px; margin-bottom: 0px"><%= obj.title %></h4>'+
'<p style="margin-bottom: 0px"><%= obj.topic %></p></div>')

USER_RIGHT = _.template(
'<div class="user-list-item">'+
'<img src="<%= obj.avatar %>" class="img-rounded" style="margin-right: 5px; height: 30px; vertical-align: middle">'+
'<span><%= obj.name %></span><br /></div>')

USER_CACHE = {}

view_main = {
    channels: {},

    getCurrentChannel: function () {
        for (i in view_main.channels) {
            if (view_main.channels[i].selected) {
                return view_main.channels[i]
            }
        }
    },

    onSendMessage: function() {
        jt.send({
            "type": "msg",
            "msg": $("#middle-input-text").val(),
            "dest": view_main.getCurrentChannel().name
        })
        $("#middle-input-text").val("")
    },

    render: function() {
        $('#middle-input-text').keypress(function(e) {
            if(e.which == 13) {
                view_main.onSendMessage();
                e.preventDefault();
            }
        });
    },

    renderUser: function () {
        $("#chat-image").attr("src", jt.user.avatar)
        $("#left-box").delegate(".left-box-element", "click", function (e) {
            view_main.select($(this).data("name"))
            view_main.renderUsers()
        })
    },

    renderChannels: function () {
        $("#left-box").empty()
        _.each(view_main.channels, function(v, i) {
            $("#left-box").append(CHANNEL_LEFT({
                obj: v
            }))
        })
    },

    renderUsers: function() {
        $("#users-online-now").empty()
        var total = 0
        _.each(view_main.getCurrentChannel().members, function(user, y) {
            $("#users-online-now").append(USER_RIGHT({
                obj: user
            }))
            total += 1
        })
        $("#online-count").text(total)
    },

    select: function(chan) {
        _.each(view_main.channels, function(channel, x) {
            channel.selected = (x == chan)
            var sel = $("#channel-"+channel.name)

            if (channel.selected) {
                sel.show()
                $("#left-channel-"+channel.name).addClass("selected")
            } else {
                $("#left-channel-"+channel.name).removeClass("selected")
                $("#left-channel-"+chan).removeClass("left-box-activity")
                sel.hide()
            }
        })
    },

    pingChannel: function(chan) {
        if (view_main.channels[chan].selected) {
            return;
        }

        $("#left-channel-"+chan).addClass("left-box-activity")
    },

    addAction: function(dest, data) {
        $("#channel-"+dest).append(CHAT_ACTION(data))
    },

    handle: function(data) {
        if (data.type == "msg") {

            $("#channel-"+data.dest).append(CHAT_MESSAGE({
                obj: data,
                time: "12 seconds ago"
            }))
        }

        if (data.type == "action") {
            view_main.addAction(data.dest, {
                obj: data,
                color: null
            })
        }

        if (data.type == "error") {
            view_main.addAction(view_main.getCurrentChannel().name, {
                obj: {
                    action: data.msg,
                    icon: "exclamation-sign"
                },
                color: "#B80000"
            })
        }

        if (data.type == "channel") {
            $("#chat-contents").append('<div style="display: none" id="channel-'+data.name+'"></div>')
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
                color: "#5bc0de"
            })
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
        jt.setupWebSocket();
        jt.view.render()
    },

    send: function(obj) {
        jt.conn.send(JSON.stringify(obj))
    },

    onSocketClose: function (e) {
        alert("Websocket closed!");
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
                    jt.view.renderUser()
                } else {
                    alert("Could not login!");
                }
                break;
        }

        jt.view.handle(obj)
    },

    setupWebSocket: function() {
        if (window["WebSocket"]) {
            jt.conn = new WebSocket("ws://"+window.location.host+"/socket");
            jt.conn.onclose = jt.onSocketClose;
            jt.conn.onmessage = jt.onSocketMessage;
            jt.conn.onopen = function () {
                jt.send({
                    "type": "hello",
                    "username": jt.user.username,
                    "name": jt.user.name,
                    "password": "1234" // HASH lewl
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