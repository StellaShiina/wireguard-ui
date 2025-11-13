<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

type Server = {
  UUID: string
  PublicIP: string
  Port: number
  EnableIPv6: boolean
  SubnetV4: string
  SubnetV6: string
}

type Peer = {
  UUID: string
  IPv4?: string | null
  IPv6?: string | null
  Name?: string | null
}

const state = reactive<{ server: Server | null; peers: Peer[] }>({ server: null, peers: [] })
const srvMsg = ref('')
const peerMsg = ref('')
const wgStatusText = ref('WG Status: Detecting...')
const wgStatusColor = ref('text-yellow-400')
const wgDetail = ref('')
const wgDetailVisible = ref(false)
const newPeerName = ref('')

async function api(path: string, opts?: RequestInit) {
  const resp = await fetch(path, {
    ...opts,
    headers: {
      'Content-Type': 'application/json',
      ...(opts && opts.headers ? opts.headers : {}),
    },
  })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new Error((data && (data.error as string)) || `HTTP ${resp.status}`)
  return data
}

async function loadAll() {
  const chk = await api('/auth/check')
  if (!chk.authenticated) {
    location.href = '/login'
    return
  }
  const cfgs = await api('/api/v1/configs')
  state.server = cfgs.server as Server
  state.peers = (cfgs.peers || []) as Peer[]
  pollWGStatus().catch(() => {})
}

async function saveServer() {
  if (!state.server) return
  if (!confirm('Confirm saving server configuration? This will invalidate all client configurations.')) return
  srvMsg.value = ''
  try {
    const body = {
      public_ip: state.server.PublicIP?.trim() || '',
      port: Number(state.server.Port || 51821),
      enable_ipv6: !!state.server.EnableIPv6,
      subnet_v4: state.server.SubnetV4?.trim() || '',
      subnet_v6: state.server.SubnetV6?.trim() || '',
    }
    const data = await api(`/api/v1/configs/server/${state.server.UUID}`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
    srvMsg.value = (data.message as string) || 'Saved'
    await wgCall('restart')
    await loadAll()
  } catch (e: any) {
    srvMsg.value = e.message
  }
}

async function createPeer() {
  peerMsg.value = ''
  try {
    const name = newPeerName.value.trim()
    const data = await api('/api/v1/configs/peer', {
      method: 'POST',
      body: JSON.stringify({ name: name || null }),
    })
    peerMsg.value = `Peer created: ${data.peer.UUID}`
    newPeerName.value = ''
    await wgCall('restart')
    await loadAll()
  } catch (e: any) {
    peerMsg.value = e.message
  }
}

async function savePeerName(uuid: string, name: string) {
  peerMsg.value = ''
  try {
    const data = await api(`/api/v1/configs/peer/${uuid}`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    })
    peerMsg.value = (data.message as string) || 'Name saved'
    await loadAll()
  } catch (e: any) {
    peerMsg.value = e.message
  }
}

async function deletePeer(uuid: string) {
  if (!confirm('Confirm deletion of this Peer? This will invalidate the client configuration.')) return
  peerMsg.value = ''
  try {
    const data = await api(`/api/v1/configs/peer/${uuid}`, { method: 'DELETE' })
    peerMsg.value = (data.message as string) || 'Deleted'
    await loadAll()
  } catch (e: any) {
    peerMsg.value = e.message
  }
}

function downloadPeer(uuid: string) {
  window.location.href = `/api/v1/configs/peer/${uuid}`
}

async function wgCall(action: 'start' | 'stop' | 'restart') {
  try {
    const data = await api(`/api/v1/wg/${action}`, { method: 'POST' })
    srvMsg.value = (data.message as string) || ''
    await pollWGStatus()
  } catch (e: any) {
    srvMsg.value = e.message
  }
}

async function fetchWGShow() {
  try {
    const detail = await api('/api/v1/wg/show', { method: 'GET' })
    wgDetail.value = (detail.output as string) || '(No output)'
    wgDetailVisible.value = true
  } catch {
    wgDetail.value = ''
    wgDetailVisible.value = false
  }
}

async function pollWGStatus() {
  try {
    const data = await api('/api/v1/wg/status', { method: 'GET' })
    if (data.status === 'ok') {
      wgStatusText.value = 'WG Status: Running'
      wgStatusColor.value = 'text-emerald-400'
      await fetchWGShow()
    } else {
      wgStatusText.value = 'WG Status: Not Running'
      wgStatusColor.value = 'text-red-400'
      wgDetailVisible.value = false
      wgDetail.value = ''
    }
  } catch {
    wgStatusText.value = 'WG Status: Failed to retrieve'
    wgStatusColor.value = 'text-yellow-400'
    wgDetailVisible.value = false
    wgDetail.value = ''
  }
}

function logout() {
  api('/auth/logout').finally(() => (location.href = '/login'))
}

onMounted(() => {
  loadAll().catch(console.error)
  setInterval(() => {
    pollWGStatus().catch(() => {})
  }, 30000)
})
</script>

<template>
  <div class="min-h-screen bg-slate-900 text-slate-200">
    <header class="sticky top-0 bg-slate-900/80 backdrop-blur border-b border-slate-800">
      <div class="max-w-6xl mx-auto px-4 py-3">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div class="flex items-center justify-between">
            <h1 class="text-base font-semibold">WireGuard Management Panel</h1>
            <span :class="['font-semibold', wgStatusColor]" class="sm:hidden">{{ wgStatusText }}</span>
          </div>
          <div class="flex items-center gap-3">
            <span :class="['font-semibold', wgStatusColor]" class="hidden sm:block">{{ wgStatusText }}</span>
            <button class="px-3 py-2 rounded bg-violet-400 text-slate-900 font-semibold" @click="wgCall('start')">Start</button>
            <button class="px-3 py-2 rounded bg-violet-400 text-slate-900 font-semibold" @click="wgCall('stop')">Stop</button>
            <button class="px-3 py-2 rounded bg-violet-400 text-slate-900 font-semibold" @click="wgCall('restart')">Restart</button>
            <button class="px-3 py-2 rounded bg-red-400 text-slate-900 font-semibold" @click="logout">Logout</button>
          </div>
        </div>
      </div>
    </header>

    <main class="max-w-6xl mx-auto px-4 py-6 space-y-6">
      <section class="rounded-xl border border-slate-800 bg-slate-800/50 p-4">
        <h2 class="text-lg font-semibold mb-3">Server Configuration</h2>
        <div v-if="state.server" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <label class="text-sm space-y-1">
            <span>Public IP</span>
            <input v-model="state.server.PublicIP" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
          </label>
          <label class="text-sm space-y-1">
            <span>Port</span>
            <input v-model.number="state.server.Port" type="number" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
          </label>
          <label class="text-sm space-y-1">
            <span>Enable IPv6</span>
            <select v-model="(state.server as Server).EnableIPv6" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400">
              <option :value="false">No</option>
              <option :value="true">Yes</option>
            </select>
          </label>
          <label class="text-sm space-y-1">
            <span>IPv4 Subnet</span>
            <input v-model="state.server.SubnetV4" placeholder="e.g. 10.7.21.0/24" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
          </label>
          <label class="text-sm space-y-1">
            <span>IPv6 Subnet</span>
            <input v-model="state.server.SubnetV6" placeholder="e.g. fd00:7:21::/64" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
          </label>
        </div>
        <div class="mt-3 text-sky-300 min-h-5">{{ srvMsg }}</div>
        <div class="flex items-center gap-3 mt-2">
          <button class="px-3 py-2 rounded bg-sky-400 text-slate-900 font-semibold" @click="saveServer">Save Configuration</button>
          <span class="text-amber-300">Note: Changing the subnet will clear all Peers and delete their configuration files.</span>
        </div>
      </section>

      <section class="rounded-xl border border-slate-800 bg-slate-800/50 p-4">
        <h2 class="text-lg font-semibold mb-3">Peers</h2>
        <div class="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-4 items-end">
          <label class="text-sm space-y-1">
            <span>Name</span>
            <input v-model="newPeerName" placeholder="Optional" class="w-full px-3 py-2 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
          </label>
          <button class="px-3 py-2 rounded bg-sky-400 text-slate-900 font-semibold" @click="createPeer">Create Peer</button>
        </div>

        <div class="overflow-x-auto mt-4">
          <table class="w-full border-collapse">
            <thead>
              <tr class="text-sm">
                <th class="border-b border-slate-700 p-2 text-left">UUID</th>
                <th class="border-b border-slate-700 p-2 text-left">IPv4</th>
                <th class="border-b border-slate-700 p-2 text-left">IPv6</th>
                <th class="border-b border-slate-700 p-2 text-left">Name</th>
                <th class="border-b border-slate-700 p-2 text-left">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in state.peers" :key="p.UUID" class="text-sm">
                <td class="border-b border-slate-800 p-2">{{ p.UUID }}</td>
                <td class="border-b border-slate-800 p-2">{{ p.IPv4 || '' }}</td>
                <td class="border-b border-slate-800 p-2">{{ p.IPv6 || '' }}</td>
                <td class="border-b border-slate-800 p-2">
                  <input :value="p.Name || ''" @input="(e:any)=>{p.Name = e.target.value}" class="w-full px-2 py-1 rounded border border-slate-700 bg-slate-900 focus:outline-none focus:ring-2 focus:ring-sky-400" />
                </td>
                <td class="border-b border-slate-800 p-2">
                  <div class="flex gap-2">
                    <button class="px-3 py-1 rounded bg-sky-400 text-slate-900 font-semibold" @click="downloadPeer(p.UUID)">Download</button>
                    <button class="px-3 py-1 rounded bg-sky-400 text-slate-900 font-semibold" @click="savePeerName(p.UUID, p.Name || '')">Save Name</button>
                    <button class="px-3 py-1 rounded bg-red-400 text-slate-900 font-semibold" @click="deletePeer(p.UUID)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mt-3 text-sky-300 min-h-5">{{ peerMsg }}</div>
      </section>

      <section v-if="wgDetailVisible" class="rounded-xl border border-slate-800 bg-slate-800/50 p-4">
        <h2 class="text-lg font-semibold mb-3">WireGuard Detailed Status</h2>
        <pre class="whitespace-pre-wrap bg-slate-900 p-3 rounded border border-slate-700">{{ wgDetail }}</pre>
      </section>
    </main>
  </div>
</template>

<style scoped></style>
