<template>
  <video-player ref="videoPlayer" :options="playerOptions" controls :playsinline="true"
    @canplay="playerCanplay($event)" @ready="playerReadied">
  </video-player>
</template>
<script>

export default {
  name: "video-player-item",
  props: {
    url: {
      type: String,
      default: ''
    },
    imgUrl: {
      type: String,
      default: ''
    }
  },
  data () {
    return {
      //vue-video-player参数配置
      playerOptions: {
        autoplay: false, //如果true,浏览器准备好时开始回放。
        preload: 'auto', // 建议浏览器在<video>加载元素后是否应该开始下载视频数据。auto浏览器选择最佳行为,立即开始加载视频（如果浏览器支持）
        // fluid: true, // 当true时，Video.js player将拥有流体大小。换句话说，它将按比例缩放以适应其容器。
        sources: [{
          type: "video/mp4",// 这里的种类支持很多种：基本视频格式、直播、流媒体等，具体可以参看git网址项目
          src: this.url // url地址
        }],
        // poster: this.imgUrl, // 你的封面地址
        controlBar: {
          timeDivider: true,
          durationDisplay: true,
          remainingTimeDisplay: false,
          fullscreenToggle: true  // 全屏按钮
        }
      }
    }
  },
  computed: {
    player () {
      return this.$refs.videoPlayer.player
    }
  },
  created () {
  },
  mounted () {
  },
  methods: {
    onPlayerPause (player) {
      player.pause();
    },
    playerReadied (player) {
    },
    playerCanplay (player) {
    },
    currentVideoPause () {
      this.$refs.videoPlayer.player.pause();
    },
    currentVideoPlay () {
      this.$refs.videoPlayer.player.play();
    },
  }
}
</script>