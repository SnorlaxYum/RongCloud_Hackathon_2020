import { Injectable } from '@angular/core';
import RongIMLib from './RongIMLib-3.0.7-dev.es.js'
import { Store } from '@ngrx/store'
import { userInfo, conversation, message } from './data'
import { HttpClient } from '@angular/common/http';

@Injectable({
  providedIn: 'root'
})
export class RongCloudService {
  conversationList = []
  finalTargetInfos = []
  newMes = []
  watched: boolean
  im: any

  rongInit(finalUserInfo: userInfo) {
    const rongConfig = {
      appkey: '25wehl3u20uaw',
    }, im = RongIMLib.init(rongConfig), that = this
    this.getConversation().subscribe(res => {
      if (res['status'] == 'success') {
        that.conversationList = res['conversations']
        that.finalTargetInfos = res['targetInfos']
        if (!that.watched) {
          that.watch(im)
          that.watched = true
        }
      }
    })
    this.im = im
    // .then(function(user) {
    //   console.log('链接成功, 链接用户 id 为: ', user.id)
    //   that.store.dispatch({ type: 'Logging into Rongcloud IM success' })
    //   window['rongIm'] = im
    // }).catch(function(error) {
    //   console.log('链接失败: ', error.code, error.msg);
    // })
  }

  getConversation() {
    return this.http.get('/api/getConversation')
  }

  updateConversation(con: conversation[]) {
    return this.http.post('/api/updateConversation', con)
  }

  getConversationMessages(mes: any) {
    return this.http.post('/api/getConversationMessages', mes)
  }

  sendMessage(mes: message) {
    return this.http.post('/api/sendMessage', mes)
  }

  editMessage(mes: message) {
    return this.http.post('/api/editMessage', mes)
  }

  recallMessage(mes: message) {
    return this.http.post('/api/recallMessage', mes)
  }

  readConversation(conv: conversation) {
    return this.http.post('/api/readConversation', conv)
  }

  read(conv: conversation) {
    var that = this
    this.readConversation(conv).subscribe(res => {
      if (res['status'] == 'success') {
        var conversation = that.im.Conversation.get({
          targetId: conv.targetId,
          type: RongIMLib.CONVERSATION_TYPE.PRIVATE
        });
        conversation.read().then(function(){
          console.log('清除未读数成功'); // im.watch conversation 将被触发
          conv.unreadMessageCount = 0
          let updatedConversationList = [conv]
          that.conversationList = that.im.Conversation.merge({
            conversationList: that.conversationList,
            updatedConversationList
          })
        });        
      }
    })
  }

  watch(im: any) {
    var that = this
    im.watch({
      conversation: function(event){
        var updatedConversationList = event.updatedConversationList; // 更新的会话列表
        that.updateConversation(updatedConversationList).subscribe(res => {
          if (res['status'] == 'success') {
            console.log('更新会话汇总:', updatedConversationList);
            that.conversationList = im.Conversation.merge({
              conversationList: that.conversationList,
              updatedConversationList
            })
            console.log('最新会话列表:', that.conversationList);
            if (that.newMes.length) {
              const notifyList = [...that.newMes]
              for (let message of notifyList) {
                console.log(message)
                that.notify(that.finalTargetInfos[message.targetId]['nickname'], message.content)
              }
              that.newMes = []
            }
          }
        })
        
      },
      message: function(event){
        var message = event.message;
        console.log('收到新消息:', message);
        that.newMes.push(event.message)
      },
      status: function(event){
        var status = event.status;
        console.log('连接状态码:', status);
      },
      expansion: function(event) {
        var updatedExpansion = event.updatedExpansion;
        var deletedExpansion = event.deletedExpansion;
        console.log('消息扩展已更新', updatedExpansion);
        /*
          {
            expansion: { key: 'value' }, // 设置或更新的扩展值
            messageUId: 'URIT-URIT-ODMF-DURR' // 设置或更新扩展的消息 uid
          }
        */
        console.log('消息扩展被删除', deletedExpansion);
        /*
          {
            deletedKeys: ['key1', 'key2'], // 删除的扩展键值集合
            messageUId: 'URIT-URIT-ODMF-DURR' // 删除扩展的消息 uid
          }
        */
      },
      chatroom: function(event) {
        var updatedEntries = event.updatedEntries;
        console.log('聊天室 KV 存储监听更新:', updatedEntries);
        /* 
          [{
            "key": "name",
            "value": "我是小融融",
            "timestamp": 1597591258338, 
            "chatroomId": "z002", 
            "type": 1 // 1: 更新（ 含:修改和新增 ）、2: 删除
        }]
        */
      }
    })
  }

  notify(user: string, message: any) {
    if (!("Notification" in window)) {
      alert("This browser does not support desktop notification");
    }
  
    // Let's check whether notification permissions have already been granted
    else if (Notification.permission === "granted") {
      // If it's okay let's create a notification
      var notification = new Notification(user, {body: message.content})
    }
  
    // Otherwise, we need to ask the user for permission
    else if (Notification.permission !== "denied") {
      Notification.requestPermission().then(function (permission) {
        // If the user accepts, let's create a notification
        if (permission === "granted") {
          var notification = new Notification(user, {body: message.content})
        }
      });
    }
  }

  constructor(private store: Store, private http: HttpClient) { }
}
