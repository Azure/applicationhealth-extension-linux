# Dev Container Info

This directory contains files to support running the code in a dev container.  This allows you build and debug the code locally, either on a PC, Mac or linux machine.

This works using VSCode's dev container support.

# Requirements

1. Docker – needed for building the docker image and for devcontainer workflow.  Microsoft has an enterprise agreement with docker, details [here](https://microsoft.service-now.com/sp?id=sc_cat_item&sys_id=234197ba1b418d54bba22173b24bcbf0) 
    - Windows Installer is [here](https://docs.docker.com/desktop/install/windows-install/)
    - Mac installer is [here](https://docs.docker.com/desktop/install/mac-install/)

1. Dev Containers VSCode extension
    - installation info is [here](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

# Running In DevContainer

This works either on windows/mac machine or via a remote SSH session on a linux machine (this is the only way that the integration tests can reliably run, debugging works in all modes)

1. Go to the root of the repo in a command window
1. Open vscode using `code .`
1. Click the Blue `><` thing in thr bottom left of the screen and select `reopen in container` (the first time this runs it will take some time as it will build the docker container for the dev environment based on the dockerfile in the .devcontainer directory)
1. Once it has opened, open a bash terminal in vscode
1. run `make devcontainer`
1. you are now ready to run and debug the extension

## Debugging

1. configure the appropriate settings in the file `.devcontainer/extension-settings.json` (the default one enables the `simple` and `process` tests for vmwatch but you can change it)
1. click the debug icon on the left and select `devcontainer run - enable` target
    - you can add more in `luanch.json` as needed
1. set breakpoints as required
1. hit f5 to launch the extension code
