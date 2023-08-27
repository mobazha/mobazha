<template>
  <div class="verifiedMod">
<%
    const badgeFrag = `<div class="badge" style="background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');" tabindex="0"></div>`;
    const warnFrag = `<div class="warning"><i class="ion-alert-circled clrTAlert"></i></div>`;
    const arrowBoxColor = ob.verified ? 'clrBrAlert2 clrBAlert2Grad' : 'clrP clrBr';
    const arrowBoxClass = `${ob.arrowClass} ${arrowBoxColor}`;

    const arrowBox = `
      <div class="arrowBox ${arrowBoxClass}">
        <div class="titleWrapper ${ob.titleWrapperClass}">
          ${ob.verified ? badgeFrag : warnFrag}
          <div class="${ob.tipTitleClass}">${ob.tipTitle}</div>
        </div>
        <div class="${ob.tipBodyClass}">${ob.tipBody}</div>
      </div>    
    `;

    let icon = `<i class="${ob.infoIconClass}"></i>`;

    if (ob.wrapInfoIcon) {
      icon = `
       <div class="arrowBoxTipWrap">
         ${icon}
         ${arrowBox}
       </div>
      `;
    }

    const textWrapper = `
      <div class="textWrapper ${ob.textWrapperClass}">
        ${ob.verified ? badgeFrag : warnFrag}
        ${
          ob.text ?
            `<span class="${ob.textClass}">${ob.text}${icon}</span>` : ''
         }
      </div>
    `;    
%>

<div :class="`innerWrap ${ob.verified ? 'verified' : 'unverified'}`">
  <div :class="`arrowBoxTipWrap`" v-if="!ob.wrapInfoIcon">
    <div :class="`textWrapper ${ob.textWrapperClass}`">
      <div v-if="ob.verified" class="badge" :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`" tabindex="0"></div>
      <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>
      
      <span v-if="ob.text" :class="ob.textClass">${ob.text}<i :class="ob.infoIconClass"></i></span>
    </div>

    <div :class="`arrowBox ${arrowBoxClass}`">
      <div :class="`titleWrapper ${ob.titleWrapperClass}`">
        <div v-if="ob.verified" class="badge" :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`" tabindex="0"></div>
        <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

        <div :class="ob.tipTitleClass">{{ ob.tipTitle }}</div>
      </div>
      <div :class="ob.tipBodyClass">{{ ob.tipBody }}</div>
    </div>
  </div>
  <div v-else>
    <div :class="`textWrapper ${ob.textWrapperClass}`">
      <div v-if="ob.verified" class="badge" :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`" tabindex="0"></div>
      <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>
      
      <span v-if="ob.text" :class="ob.textClass">${ob.text}
        <div class="arrowBoxTipWrap">
          <i class="${ob.infoIconClass}"></i>

          <div class="arrowBox ${arrowBoxClass}">
            <div class="titleWrapper ${ob.titleWrapperClass}">
              <div v-if="ob.verified" class="badge" :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`" tabindex="0"></div>
              <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

              <div :class="ob.tipTitleClass">{{ ob.tipTitle }}</div>
            </div>
            <div :class="ob.tipBodyClass">{{ ob.tipBody }}</div>
          </div>    
       </div>
      </span>
    </div>
  </div>
</div>


</div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

</script>
<style lang="scss" scoped>
</style>