var TEMPLATES = {

    CHAT_CONTENT: _.template(
        '<p style="<% if (!obj.nofloat) { %>float: right; <% } %>"><%= time %><br /><span><%= obj.msg %></span></p>'),

    CHAT_MESSAGE: _.template(
        '<div data-author="<%= obj.username %>" class="message-box <%= obj.highlight ? "highlight" : "" %>">'+
        '<img src="<%= obj.avatar %>" class="img-rounded chat-avatar"><b><%= obj.name %></b>'+
        '<%= content %></div>'),

    CHAT_ACTION: _.template(
        '<div class="message-box message-box-action">'+
        '<% if (obj.icon) { %><i class="fa fa-<%= obj.icon %>"></i><% } %>'+
        '<i style="margin-left: 10px; <% if (color) {%>color: <%= color %> <% } %>">'+
        '<% if (obj.raw) { %><%= obj.action %><% } else {%><%- obj.action %><% } %></i></div>'),

    CHANNEL_LEFT: _.template(
        '<div id="left-channel-<%= obj.name %>" class="left-box-element <% if (obj.selected) { %>selected<% } %>" data-name="<%= obj.name %>">'+
        '<img src="<%= obj.image %>" class="img-rounded" style="float: left; margin-right: 5px; height: 40px; width: 40px;">'+
        '<h4 style="margin-top: 0px; margin-bottom: 0px"><%= obj.title %></h4>'+
        '<p style="margin-bottom: 0px"><%= obj.topic %></p></div>'),

    USER_RIGHT: _.template(
        '<div class="user-list-item">'+
        '<img src="<%= obj.avatar %>" class="img-rounded <%= obj.afk ? "user-afk" : "user-active" %>" '+
        'style="margin-right: 5px; height: 45px; vertical-align: middle">'+
        '<span><%= obj.name %></span><br /></div>'),

    NOTIFICATION: _.template("<%- username %>: <%= msg %><% if (count) { %>"+
        " (and <%= count %> more)<% } %>"),

    MENU_ITEM: _.template('<li><a data-key="<%= key %>" class="toggle" id="toggle-<%= key %>"><%= value ? "Disable" : "Enable" %> <%= key %></a></li>')
}
