# **Node Refiner Operator**

[![REUSE status](https://api.reuse.software/badge/github.com/SAP/node-refiner)](https://api.reuse.software/info/github.com/SAP/node-refiner)

## Description
In long-running clusters (production clusters) we often can improve the efficiency of resource usage by rescheduling workloads from under-utilized nodes.

This job can be partly performed by the cluster autoscaler (**CA**) if we look at each node separately - this can be done by setting a utilization threshold that the **CA** respects, as long as the node achieves utilization metric above that threshold the node stays, otherwise the **CA** will attempt to remove this node from the cluster.

The above mentioned solution might sound like it solves the problem of under-utilized nodes, but lets take another scenario as an example.

### Pitfall
Lets assume that the utilization metric is set to 50%, that means if either the usage of RAM or CPU resources of the node is above 50% - the node stays.

![Node Utilization](docs/img/node-util.png)

In this figure we can see that even though the utilization target of over 50% is achieved, there is wasted resources that can be terminated without affecting the delivery of our tasks.

### Proposed Solution

We introduce the notion of excess nodes. By calculating the excess resources from each node in the cluster and combine them together, we get a holistic view on how many resources can potentially be saved.

![Excess Utilization](docs/img/excess-calc.png)


### Node Refiner Process
1. Node Refiner (**NR**) determines the node with the largest potential to be terminated (the one with the least utilization metrics) and elects it as a potential node to drain.
2. **NR** cordons the node to avoid new pods being scheduled on this node while the operator is evicting the existing pods on this node.
3. Pods are being gracefully terminated in parallel. In case any of the pods have conditions that do not allow eviction the draining process halts and the node is uncordoned.
4. If all the Pods are succesfully evicted, **NR** will then leave the node cordoned; the cluster autoscaler should consequently pick that this node as it is under-utilized and needs to be deleted.
5. As long as there are excess nodes above a certain configurable threshold the process is repeated.

### NR Conditions
Each cluster might need its own set of conditions to run NR efficiently. The following configurations allow to adjust the behavior of NR to be more relaxed (deliver jobs in a large amount of nodes) or aggressive (deliver jobs in the least amount of nodes) depending on the clusters needs.

| Configuration | Config Map Variable | Description | Default Value |
|:----:|-----|-----------|-----------|
|**CalculationLoopFrequency**| Not Configured |Time to recalculate the cluster utilization metrics| 1m |
|**DefaultMinimumTimeSinceLastAddition**|time_since_last_addition|Grace period after a node is added to the cluster to ensure that no draining of nodes takes place before the cluster stabilizes its resources|60m|
|**DefaultTimeGap**|time_gap|Default time between node drains, or if a node fails to drain it's the time before another retry takes place|10m|
|**DefaultMinimumNodes**|minimum_nodes|The minimum number of nodes that should be in the cluster|2|
|**DefaultMinimumNonTaintedNodes**|minimum_non_tainted_nodes|The minimum number of non-tainted nodes that should be in the cluster|2|
|**DefaultExcessNodes**|excess_nodes_threshold|If the number of excess nodes in the cluster exceeds this number a scale down takes place.|2|
|**DrainerEnabled**|drainer_enabled|Flag for enabling the drainer to take any actions. Set to False for "dry run" mode|True|

### Summary
**Node Refiner (NR)** aims to collect information about the cluster by aggregating all the nodes and pods metrics to build an overview of the cluster utilization. By analyzing this information, we can make an informed decision on whether we should remove some of the existing nodes or not. 

Also, it allows us to pick the node with lowest utilization metrics, making sure that we remove the node that will cause the smallest disturbance in the availability of our cluster. **NR** calculates the metrics. When a removal process is about to occur, **NR** drains the node gracefully and outsources the deletion process of the node to the **Cluster Autoscaler**; we took that decision for two reasons.

1.  To not rewrite redundant code since the **cluster autoscaler** already has that functionality fulfilled.
2.  **CA** notifies a lot of subscribers to the event of a removal of node (ex. Gardener), thus making sure that we do not fall in a limbo of removing/adding node; therefore, we found that it's the most stable when the **CA** handles that part of the process.

### Process Summary
1. Gather information about the existing pods in the cluster and analyze their requests/usage
2. Analyze the cluster capacity and whether it can satisfy the pods requirements with less nodes
3. Analyze the individual nodes and check whether any of them can be evicted.
4. Drain under-utilized node gracefully

## Documentation

- [Quickstart](docs/quickstart.md) 
- [Testing](docs/testing.md)
- [Monitoring](docs/monitoring.md)


## Contributing

Please check the [Development Conventions and Guidelines](CONTRIBUTING.md) document.

## Code of Conduct

Everyone participating in this joint project is welcome as long as our [Code of Conduct](CODE_OF_CONDUCT.md) is being adhered to.

## Support

Feel free to open new issues for feature requests, bugs or general feedback on the [GitHub issues page of this project](https://github.com/SAP/node-refiner/issues).

## Licensing

Copyright (2022) SAP SE and node-refiner contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available via the REUSE tool.