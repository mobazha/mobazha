<template>
  <div class="home-TUIKit-main">
    <!-- <div
      :class="env?.isH5 ? 'conversation-h5' : 'conversation'"
      v-show="!env?.isH5 || currentModel === 'conversation'"
    >
      <TUISearch class="TUI-search" />
      <TUIConversation @current="handleCurrentConversation" />
    </div> -->
    <div class="chat" v-show="!env?.isH5 || currentModel === 'message'">
      <TUIChat :conversationID="conversationID">
        <h1>欢迎使用腾讯云即时通信</h1>
      </TUIChat>
    </div>
    <!-- TUICallKit 组件：通话 UI 组件主体 -->
    <!-- <TUICallKit
      :class="!showCallMini ? 'callkit-drag-container' : 'callkit-drag-container-mini'"
      :allowedMinimized="true"
      :allowedFullScreen="false"
      :beforeCalling="beforeCalling"
      :afterCalling="afterCalling"
      :onMinimized="onMinimized"
      :onMessageSentByMe="onMessageSentByMe"
    /> -->
  </div>
</template>

<script lang="ts">
import { defineComponent, reactive, toRefs } from "vue";
import { TUIEnv } from "../TUIKit/TUIPlugin";
import { handleErrorPrompts } from "../TUIKit/TUIComponents/container/utils";

export default defineComponent({
  name: "Chat",
  props: {
    conversationID: {
      type: String,
      default: '',
    },
  },
  setup(props) {
    const data = reactive({
      env: TUIEnv(),
      currentModel: "conversation",
      showCall: false,
      showCallMini: false,
      conversationID: props.conversationID,
    });
    const TUIServer = (window as any)?.TUIKitTUICore?.TUIServer;
    const handleCurrentConversation = (value: string) => {
      data.currentModel = value ? "message" : "conversation";
    };
    // beforeCalling：在拨打电话前与收到通话邀请前执行
    const beforeCalling = (type: string, error: any) => {
      if (error) {
        handleErrorPrompts(error, type);
        return;
      }
      data.showCall = true;
    };
    // afterCalling：结束通话后执行
    const afterCalling = () => {
      data.showCall = false;
      data.showCallMini = false;
    };
    // onMinimized：组件切换最小化状态时执行
    const onMinimized = (
      oldMinimizedStatus: boolean,
      newMinimizedStatus: boolean
    ) => {
      data.showCall = !newMinimizedStatus;
      data.showCallMini = newMinimizedStatus;
    };
    // onMessageSentByMe：在整个通话过程内发送消息时执行
    const onMessageSentByMe = async (message: any) => {
      TUIServer?.TUIChat?.handleMessageSentByMeToView(message);
      return;
    };
    return {
      ...toRefs(data),
      handleCurrentConversation,
      beforeCalling,
      afterCalling,
      onMinimized,
      onMessageSentByMe,
    };
  },
});
</script>
<style scoped>
.home-TUIKit-main {
  display: flex;
  height: 100vh;
  overflow: hidden;
}
.TUI-search {
  padding: 12px;
}
.conversation {
  min-width: 285px;
  flex: 0 0 24%;
  border-right: 1px solid #f4f5f9;
}
.conversation-h5 {
  flex: 1;
  border-right: 1px solid #f4f5f9;
}
.chat {
  flex: 1;
  height: 85%;
  position: relative;
}
.callkit-drag-container {
  position: fixed;
  left: calc(50% - 25rem);
  top: calc(50% - 18rem);
  width: 50rem;
  height: 36rem;
  border-radius: 16px;
  box-shadow: rgba(0, 0, 0, 0.16) 0px 3px 6px, rgba(0, 0, 0, 0.23) 0px 3px 6px;
}
.callkit-drag-container-mini {
  position: fixed;
  width: 168px;
  height: 56px;
  right: 10px;
  top: 70px;
}
</style>