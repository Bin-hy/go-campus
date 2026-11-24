<script setup lang="ts">
import { computed } from 'vue'
import { annotationBridge } from './annotation-bridge'

const initials = computed(() => {
  const name = (annotationBridge.user?.displayName || '').trim()
  if (!name) return '?'
  return [...name][0].toUpperCase()
})

function openDrawer(): void {
  annotationBridge.actions.openDrawer()
}

function openAccount(): void {
  annotationBridge.actions.openAccount()
}
</script>

<template>
  <div v-if="annotationBridge.hasDoc" class="annotation-nav">
    <button
      v-if="annotationBridge.status === 'error'"
      class="annotation-nav-btn"
      type="button"
      disabled
      :title="annotationBridge.error || '批注服务暂不可用'"
    >
      批注不可用
    </button>
    <template v-else>
      <button
        class="annotation-nav-btn"
        type="button"
        :disabled="annotationBridge.status !== 'ready'"
        :aria-label="`批注 ${annotationBridge.count}`"
        @click="openDrawer"
      >
        批注
        <span v-if="annotationBridge.count > 0" class="annotation-nav-count">
          {{ annotationBridge.count }}
        </span>
      </button>

      <button
        v-if="!annotationBridge.user"
        class="annotation-nav-btn"
        type="button"
        :disabled="annotationBridge.status !== 'ready'"
        @click="openAccount"
      >
        登录
      </button>
      <button
        v-else
        class="annotation-nav-user"
        type="button"
        :aria-label="`账号：${annotationBridge.user.displayName}`"
        @click="openAccount"
      >
        <img
          v-if="annotationBridge.user.avatarUrl"
          class="annotation-nav-avatar"
          :src="annotationBridge.user.avatarUrl"
          alt=""
        />
        <span v-else class="annotation-nav-avatar fallback">{{ initials }}</span>
        <span class="annotation-nav-name">{{ annotationBridge.user.displayName }}</span>
      </button>
    </template>
  </div>
</template>

<style scoped>
.annotation-nav {
  display: none;
  align-items: center;
  gap: 4px;
  margin-left: 4px;
}

@media (max-width: 767px), (min-width: 1280px) {
  .annotation-nav {
    display: flex;
  }
}

.annotation-nav-btn,
.annotation-nav-user {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 5px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--vp-c-text-1);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease;
}

.annotation-nav-btn:hover:not(:disabled),
.annotation-nav-user:hover {
  color: var(--vp-c-brand-1);
  background: var(--vp-c-bg-soft);
}

.annotation-nav-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.annotation-nav-btn:focus-visible,
.annotation-nav-user:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 2px;
}

.annotation-nav-count {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--vp-c-brand-1);
  color: #fff;
  font-size: 11px;
  line-height: 18px;
  text-align: center;
}

.annotation-nav-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  object-fit: cover;
  background: var(--vp-c-bg-soft);
}

.annotation-nav-avatar.fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--vp-c-brand-1);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.annotation-nav-name {
  display: none;
}

@media (max-width: 768px) {
  .annotation-nav {
    gap: 2px;
    margin-left: 0;
  }
}
</style>
