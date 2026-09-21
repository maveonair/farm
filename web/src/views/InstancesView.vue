<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { State } from '@/api/client'
import InstanceTable from '@/components/InstanceTable.vue'
import PaginationControls from '@/components/PaginationControls.vue'
import ViewState from '@/components/ViewState.vue'
import { states } from '@/lib/lifecycle'
import { pageFrom } from '@/lib/pagination'
import { useInstancesQuery } from '@/queries/instances'
import { queryError } from '@/queries/options'
import { usePoolOptionsQuery } from '@/queries/pools'

const route = useRoute()
const router = useRouter()

const pool = computed({
  get: () => (typeof route.query.pool === 'string' ? route.query.pool : ''),
  set: (value: string) => {
    const query = { ...route.query }
    delete query.page
    if (value) {
      query.pool = value
    } else {
      delete query.pool
    }

    void router.replace({ query })
  },
})

const state = computed<State | ''>({
  get: () => {
    const value = typeof route.query.state === 'string' ? route.query.state : ''
    return states.includes(value as State) ? (value as State) : ''
  },
  set: (value) => {
    const query = { ...route.query }
    delete query.page
    if (value) {
      query.state = value
    } else {
      delete query.state
    }

    void router.replace({ query })
  },
})

const poolsQuery = usePoolOptionsQuery()
const page = computed(() => pageFrom(route.query.page))
const instancesQuery = useInstancesQuery(pool, state, page)

const pools = computed(() => poolsQuery.data.value ?? [])
const instances = computed(() => instancesQuery.data.value?.items ?? [])
const pagination = computed(() => instancesQuery.data.value?.pagination)
const loading = instancesQuery.isPending
const error = computed(() =>
  queryError(instancesQuery.error.value, instancesQuery.data.value, 'Unable to load instances'),
)
</script>

<template>
  <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
    <div>
      <h1 class="page-title mt-1">Instances</h1>
    </div>
    <div class="flex gap-2">
      <select v-model="pool" class="field">
        <option value="">All pools</option>
        <option v-for="item in pools" :key="item.name" :value="item.name">
          {{ item.name }}
        </option></select
      ><select v-model="state" class="field capitalize">
        <option value="">All states</option>
        <option v-for="item in states" :key="item" :value="item">{{ item }}</option>
      </select>
    </div>
  </div>
  <ViewState :loading :error :empty="!instances.length && page === 1"
    ><template #content
      ><InstanceTable :instances /> <PaginationControls v-if="pagination" :pagination /></template
    ><span>No matching instances.</span></ViewState
  >
</template>
