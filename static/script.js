
CHAT_MESSAGE = _.template(
    '<div class="message-box">'+
    '<img src="<%= obj.avatar %>" class="img-rounded chat-avatar"><b><%= obj.username %></b><br />'+
    '<span><%= obj.msg %></span></div>')

CHAT_ACTION = _.template(
'<div class="message-box message-box-action">'+
    '<% if (obj.icon) { %><i class="glyphicon glyphicon-<%= obj.icon %>"></i><% } %>'+
    '<i style="margin-left: 10px; <% if (color) {%>color: <%= color %> <% } %>"><%= obj.action %></i></div>')


USER_CACHE = {}

view_main = {
    onSendMessage: function() {
        jt.send({
            "type": "msg",
            "msg": $("#middle-input-text").val(),
            "dest": "lobby"
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

    handle: function(data) {
        if (data.type == "msg") {
            $("#chat-contents").append(CHAT_MESSAGE({
                obj: data,
                time: "12 seconds ago"
            }))
        }

        if (data.type == "action") {
            $("#chat-contents").append(CHAT_ACTION({
                obj: data,
                color: null
            }))
        }

        if (data.type == "error") {
            $("#chat-contents").append(CHAT_ACTION({
                obj: {
                    action: data.msg,
                    icon: "exclamation-sign"
                },
                color: "#B80000"
            }))
        }
    }
}

// No, not justin... never justin...
jt = {
    conn: null,
    user: {
        username: "test1",
        authed: false
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
                    jt.user.authed = true;
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