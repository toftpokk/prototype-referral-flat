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