<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    v-bind="$attrs"
  ><defs><linearGradient
    :id="`${uid}-mimo-color-bg`"
    x1="4"
    x2="20"
    y1="3"
    y2="21"
    gradientUnits="userSpaceOnUse"
  ><stop
    offset="0%"
    stop-color="#FF8A00"
  /><stop
    offset="58%"
    stop-color="#FF4D6D"
  /><stop
    offset="100%"
    stop-color="#7C4DFF"
  /></linearGradient><linearGradient
    :id="`${uid}-mimo-color-orb`"
    x1="16.3"
    x2="19.1"
    y1="5.6"
    y2="8.4"
    gradientUnits="userSpaceOnUse"
  ><stop
    offset="0%"
    stop-color="#8FE8FF"
  /><stop
    offset="100%"
    stop-color="#DFF9FF"
  /></linearGradient></defs><rect
    x="2"
    y="2"
    width="20"
    height="20"
    rx="6"
    :fill="`url(#${uid}-mimo-color-bg)`"
  /><path
    d="M6.75 15.75v-6.07c0-1.03.83-1.86 1.86-1.86.63 0 1.22.32 1.56.86L12 11.29l1.83-2.61c.34-.54.93-.86 1.56-.86 1.03 0 1.86.83 1.86 1.86v6.07a1 1 0 1 1-2 0v-3.84l-1.72 2.45a1.86 1.86 0 0 1-3.06 0L8.75 11.9v3.85a1 1 0 1 1-2 0Z"
    fill="#fff"
  /><circle
    cx="17.7"
    cy="6.9"
    r="1.7"
    :fill="`url(#${uid}-mimo-color-orb)`"
  /></svg>
</template>

<script setup lang="ts">
import { useId } from 'vue'

withDefaults(defineProps<{ size?: string | number }>(), { size: '1em' })
defineOptions({ inheritAttrs: false })

const uid = useId()
</script>
