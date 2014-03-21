var TEMPLATES = {
    CHAT_MESSAGE: _.template(
        '<div class="message-box">'+
        '<img src="<%= obj.avatar %>" class="img-rounded chat-avatar"><b><%= obj.name %></b>'+
        '<p style="float: right;"><%= time %><br /><span><%= obj.msg %></span></div>'),

    CHAT_ACTION: _.template(
        '<div class="message-box message-box-action">'+
        '<% if (obj.icon) { %><i class="fa fa-<%= obj.icon %>"></i><% } %>'+
        '<i style="margin-left: 10px; <% if (color) {%>color: <%= color %> <% } %>"><%- obj.action %></i></div>'),

    CHANNEL_LEFT: _.template(
        '<div id="left-channel-<%= obj.name %>" class="left-box-element <% if (obj.selected) { %>selected<% } %>" data-name="<%= obj.name %>">'+
        '<img src="<%= obj.image %>" class="img-rounded" style="float: left; margin-right: 5px; height: 40px; width: 40px;">'+
        '<h4 style="margin-top: 0px; margin-bottom: 0px"><%= obj.title %></h4>'+
        '<p style="margin-bottom: 0px"><%= obj.topic %></p></div>'),

    USER_RIGHT: _.template(
        '<div class="user-list-item">'+
        '<img src="<%= obj.avatar %>" class="img-rounded" style="margin-right: 5px; height: 30px; vertical-align: middle">'+
        '<span><%= obj.name %></span><br /></div>'),

    NOTIFICATION: _.template("<%- username %>: <%= msg %><% if (count) { %>"+
        " (<%= count %> unread messages)<% } %>")
}
