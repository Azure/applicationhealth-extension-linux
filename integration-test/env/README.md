# Integration Test Environment

This directory holds a skeleton of what we copy to `/var/lib/waagent` in the
integration testing Docker image.

```
.
├── {THUMBPRINT}.crt                <-- tests generate and push this certificate
├── {THUMBPRINT}.prv                <-- tests generate and push this private key
├── Extension/                  
├   ├── HandlerManifest.json        <-- docker image build pushes it here
├   ├── HandlerEnvironment.json     <-- the extension reads this
├   ├── bin/                        <-- docker image build pushes the extension binary here
├   ├   └── VMWatch/
├   ├       ├── vmwatch_linux_amd64 <-- VMWatch AMD64 binary
├   ├       └── vmwatch.conf        <-- VMWatch configuration file
├───├── config/                     <-- tests push 0.settings file here
└───└── status/                     <-- extension should write here
```