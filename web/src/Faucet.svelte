<script>
  import { onMount } from 'svelte';
  import { getAddress } from '@ethersproject/address';
  import { CloudflareProvider } from '@ethersproject/providers';
  import { setDefaults as setToast, toast } from 'bulma-toast';

  let input = null;
  let faucetInfo = {
    FaucetEOAAddress: '0x0000000000000000000000000000000000000000',
    NetworkName: 'testnet',
    ETHAmount: 1,
  };

  $: document.title = `${capitalize(faucetInfo.NetworkName)} Faucet`;

  onMount(async () => { 
    const res = await fetch('/api/info');
    faucetInfo = await res.json();
  });

  setToast({
    position: 'bottom-center',
    dismissible: true,
    pauseOnHover: true,
    closeOnClick: false,
    animate: { in: 'fadeIn', out: 'fadeOut' },
  });

  async function handleRequest() {
    let address = input;
    if (address.endsWith('.eth')) {
      try {
        const provider = new CloudflareProvider();
        address = await provider.resolveName(address);
        if (!address) {
          toast({ message: 'invalid ENS name', type: 'is-warning' });
          return;
        }
      } catch (error) {
        toast({ message: error.reason, type: 'is-warning' });
        return;
      }
    }

    try {
      address = getAddress(address);
    } catch (error) {
      toast({ message: error.reason, type: 'is-warning' });
      return;
    }

    const res = await fetch('/api/claim', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        address,
      }),
    });

    let { msg } = await res.json();
    let type = res.ok ? 'is-success' : 'is-warning';
    toast({ message: msg, type });
  }

  function capitalize(str) {
    const lower = str.toLowerCase();
    return str.charAt(0).toUpperCase() + lower.slice(1);
  }
</script>

<main>
  <section class="hero is-info is-fullheight">
    <div class="hero-body">
      <div class="container has-text-centered">
        <div class="column is-6 is-offset-3">
          <h1 class="title">
            Request {faucetInfo.ETHAmount/1000000000000000000} Native Token
          </h1>
          <h2 class="subtitle">
            Serving from {faucetInfo.FaucetEOAAddress}
          </h2>
          <div class="box">
            <div class="field is-grouped">
              <p class="control is-expanded">
                <input
                  bind:value={input}
                  class="input is-rounded"
                  type="text"
                  placeholder="Enter your address or ENS name"
                />
              </p>
              <p class="control">
                <button
                  on:click={handleRequest}
                  class="button is-primary is-rounded"
                >
                  Request
                </button>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</main>

<style>
  .hero.is-info {
    background: url('/background.jpg') no-repeat center center fixed;
    -webkit-background-size: cover;
    -moz-background-size: cover;
    -o-background-size: cover;
    background-size: cover;
  }
  .hero .subtitle {
    padding: 3rem 0;
    line-height: 1.5;
  }
  .box {
    border-radius: 19px;
  }
</style>
