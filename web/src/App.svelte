<script lang="ts">
  import { onMount } from 'svelte';

  let tunnels: any[] = [];
  let peers: any[] = [];
  let policies: any[] = [];
  let loading = true;

  async function fetchData() {
    try {
      const [tunnelsRes, peersRes, policiesRes] = await Promise.all([
        fetch('/api/tunnels'),
        fetch('/api/peers'),
        fetch('/api/policies'),
      ]);

      tunnels = await tunnelsRes.json();
      peers = await peersRes.json();
      policies = await policiesRes.json();
    } catch (error) {
      console.error('Failed to fetch data:', error);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    fetchData();
    // Refresh every 5 seconds
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  });

  function getStateColor(state: string): string {
    switch (state) {
      case 'established':
        return 'text-green-600';
      case 'connecting':
        return 'text-yellow-600';
      case 'down':
      case 'error':
        return 'text-red-600';
      default:
        return 'text-gray-600';
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  }
</script>

<main class="min-h-screen bg-gray-50">
  <header class="bg-white shadow">
    <div class="max-w-7xl mx-auto py-6 px-4">
      <h1 class="text-3xl font-bold text-gray-900">
        IPsec Manager Dashboard
      </h1>
      <p class="text-gray-600 mt-1">SWAVLAMBAN 2025 - Unified Cross-Platform IPsec Solution</p>
    </div>
  </header>

  <div class="max-w-7xl mx-auto py-6 px-4">
    {#if loading}
      <div class="flex justify-center items-center h-64">
        <div class="text-gray-500">Loading...</div>
      </div>
    {:else}
      <!-- Stats Overview -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div class="bg-white rounded-lg shadow p-6">
          <h3 class="text-gray-500 text-sm font-medium">Active Tunnels</h3>
          <p class="text-3xl font-bold text-gray-900 mt-2">
            {tunnels.filter(t => t.state === 'established').length}
          </p>
          <p class="text-sm text-gray-500 mt-1">of {tunnels.length} total</p>
        </div>

        <div class="bg-white rounded-lg shadow p-6">
          <h3 class="text-gray-500 text-sm font-medium">Connected Peers</h3>
          <p class="text-3xl font-bold text-gray-900 mt-2">
            {peers.filter(p => p.status === 'online').length}
          </p>
          <p class="text-sm text-gray-500 mt-1">of {peers.length} registered</p>
        </div>

        <div class="bg-white rounded-lg shadow p-6">
          <h3 class="text-gray-500 text-sm font-medium">Active Policies</h3>
          <p class="text-3xl font-bold text-gray-900 mt-2">
            {policies.filter(p => p.enabled).length}
          </p>
          <p class="text-sm text-gray-500 mt-1">of {policies.length} total</p>
        </div>
      </div>

      <!-- Tunnels Section -->
      <div class="bg-white rounded-lg shadow mb-8">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-xl font-semibold text-gray-900">IPsec Tunnels</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">State</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Local Address</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Remote Address</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Data In</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Data Out</th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              {#if tunnels.length === 0}
                <tr>
                  <td colspan="6" class="px-6 py-4 text-center text-gray-500">
                    No tunnels configured
                  </td>
                </tr>
              {:else}
                {#each tunnels as tunnel}
                  <tr>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                      {tunnel.name}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <span class="text-sm font-medium {getStateColor(tunnel.state)}">
                        {tunnel.state}
                      </span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {tunnel.local_address || '-'}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {tunnel.remote_address || '-'}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {formatBytes(tunnel.bytes_in || 0)}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {formatBytes(tunnel.bytes_out || 0)}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

      <!-- Peers Section -->
      <div class="bg-white rounded-lg shadow mb-8">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-xl font-semibold text-gray-900">Connected Peers</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Hostname</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Platform</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">IP Address</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Version</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Last Seen</th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              {#if peers.length === 0}
                <tr>
                  <td colspan="6" class="px-6 py-4 text-center text-gray-500">
                    No peers registered
                  </td>
                </tr>
              {:else}
                {#each peers as peer}
                  <tr>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                      {peer.hostname}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {peer.platform}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {peer.ip_address}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {peer.version}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <span class="text-sm font-medium {peer.status === 'online' ? 'text-green-600' : 'text-gray-600'}">
                        {peer.status}
                      </span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {new Date(peer.last_seen_at).toLocaleString()}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

      <!-- Policies Section -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-xl font-semibold text-gray-900">Policies</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Description</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Tunnels</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Priority</th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              {#if policies.length === 0}
                <tr>
                  <td colspan="5" class="px-6 py-4 text-center text-gray-500">
                    No policies configured
                  </td>
                </tr>
              {:else}
                {#each policies as policy}
                  <tr>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                      {policy.name}
                    </td>
                    <td class="px-6 py-4 text-sm text-gray-500">
                      {policy.description || '-'}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {policy.tunnels?.length || 0}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {policy.priority}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <span class="text-sm font-medium {policy.enabled ? 'text-green-600' : 'text-gray-600'}">
                        {policy.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }
</style>
