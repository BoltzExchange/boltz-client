# RTL Umbrel Testing

You can try the beta version of boltz-client in your umbrel rtl installation.

The update has to be performed manually in the cli of your umbrel server (`ssh umbrel@umbrel.local`):

```bash
cd umbrel
cp app-data/ride-the-lightning/boltz/.boltz-lnd/boltz.db boltz-lnd-backup.db
sudo ./scripts/repo checkout https://github.com/BoltzExchange/umbrel-apps.git
sudo ./scripts/app update ride-the-lightning
```

After updating, you should be able to open RTL and still see the old swap data and continue swapping like before.

You can also verify that `boltz-client`, and not `boltz-lnd` is running using the cli:

```bash
alias boltzcli="sudo docker exec -it ride-the-lightning_boltz_1 boltzcli"
boltzcli getinfo
boltzcli listswaps
```

If anything goes wrong, you can restore the old state:

```bash
sudo ./scripts/app stop ride-the-lightning
cp boltz-lnd-backup.db app-data/ride-the-lightning/boltz/.boltz-lnd/boltz.db
sudo ./scripts/repo checkout https://github.com/getumbrel/umbrel-apps
sudo ./scripts/app update ride-the-lightning
```

