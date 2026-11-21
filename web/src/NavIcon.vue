<script setup lang="ts">
defineProps<{name:string}>()
const paths:Record<string,string>={
 overview:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
 hosts:'M4 3h16v7H4z M4 14h16v7H4z M7 6.5h.01 M7 17.5h.01 M11 6.5h6 M11 17.5h6',
 nodes:'M12 3v5 M5 16v-4h14v4 M12 8v4 M3 16h4v5H3z M10 16h4v5h-4z M17 16h4v5h-4z',
 users:'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M16 3a4 4 0 0 1 0 8 M22 21v-2a4 4 0 0 0-3-3.87 M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
 policies:'M4 7h16 M4 17h16 M8 4v6 M16 14v6',
 subscriptions:'M10 13a5 5 0 0 0 7 .2l3-3a5 5 0 0 0-7-7l-2 2 M14 11a5 5 0 0 0-7-.2l-3 3a5 5 0 0 0 7 7l2-2',
 settings:'M4 7h16 M4 17h16 M8 4v6 M16 14v6'
}
</script>
<template><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="paths[name]??paths.overview"/></svg></template>
