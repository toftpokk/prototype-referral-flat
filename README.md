# Prototype Patient Referral System

Prototype for an online patient referral system 

## Project Structure
- / - go project tree
- /auth - authentication files
- /lib - helpers
- /referralClient - client side
- /referralServer - server side
- /data - patient data, mock HIS

## Locations & Roles
- Origin Hospital (Client)
  - Doctor
- Central (Server)
  - Patient
- Destination Hospital (Client)
  - Hospital Staff
  - Doctor
- CA (Handles Certificate Signing)

## Build & Run
```sh
cd src
go run ./cmd/referralServer

# On another terminal
cd src
go run ./cmd/referralClient
```
Note: request sending/managing is done at frontend

## Dataset
coherent-11-07-2022, remove organizations and practitioners

from https://synthea.mitre.org/downloads

# For demo

Go to cert-gen directory

  cd ./cert-gen

1. Generate root certificate

    ./ca-gen

2. Generate origin

    cp client.cnf origin.cnf

    # rename O and CN to Origin
    vim origin.cnf 

    ./client-csr-gen origin
    ./ca-sign origin

    # Save the serial number (ca.srl) in decimal form

3. Generate dest

    cp client.cnf dest.cnf

    # rename O and CN to Dest
    vim dest.cnf 

    ./client-csr-gen dest
    ./ca-sign dest

    # Save the serial number (ca.srl) in decimal form

4. Generate central

    cp client.cnf central.cnf

    # rename O and CN to Central
    vim central.cnf 

    ./client-csr-gen central
    ./ca-sign central

    # Save the serial number (ca.srl) in decimal form
  
Move ceritificates to auth

  cd ..
  mkdir auth
  mv cert-gen/*.crt cert-gen/*.key cert-gen/*.csr auth/
  

Then replace the test data's serial numbers with saved serial numbers (`fillTestData`)

After that download the coherent dataset here https://synthea.mitre.org/downloads and move into `his/`

Patients' citizen ID can be gotten from his data: form '000-00-0000'
