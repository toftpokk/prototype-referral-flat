# README

Begin by generating the root certificate (CA)

    ./ca-gen

Next, copy the client config and modify, especially the `O` and `CN` fields

    ```sh
    cp client.cnf myclient.cnf
    ```

    ```conf
    # myclient.cnf
    # ...
    O = myclient
    CA = myclient
    # ...
    ```

Then generate client CSR (Client)

    ./client-csr-gen myclient

Finally, sign (CA)

    ./ca-sign myclient

Note: for servers
```
[ v3_ca ]
subjectAltName = IP:192.168.1.1
```
