# Metal Pod Reaper

Automatically recover Stateful application Pods when a Kubernetes node (on metal) is down.

- [Details](#details)
- [Usage](#usage)
- [Build](#build)
- [Roadmap](#roadmap)

## Details

In Kubernetes, when there is no Cloud Provider integration, there is no automated way of automatically recovering workloads (pods) when the status of a node is Unknown.

## Usage

TBD

## Build

Binaries are created in `./bin/`.

To install dependencies and build:
`make`

To Build with dependencies and test:
`make test`

To build quickly:
`make build`

## Roadmap

Metal Pod Reaper releases are detailed in the
 [milestone page](https://github.com/appvia/metal-pod-reaper/milestones).
