# tutorial-03-revocation-server — VHS replay.
#
# Operator starts the revocation HTTP server from the Servers
# screen, then revokes a licence — the CRL the client polls now
# reflects the new state.

Width 144
Height 44
Delay 320ms
Output vhs/out/tutorial-03-revocation-server.gif

Sleep 600ms

# Servers tab.
Key 7
Sleep 800ms

# Switch back to licences to revoke a row — synthetic seed.
Key 2
Seed licenses 5
Sleep 700ms
Key d
Sleep 500ms
Key A
Sleep 700ms

# Revocation tab.
Key 6
Seed revocation 2
Sleep 800ms
Key d
Sleep 500ms
