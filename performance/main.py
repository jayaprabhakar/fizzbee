import json
import sys
import performance.markov_chain as markov_chain
import proto.performance_model_pb2 as perf
import performance.fmt as fmt
import argparse

import matplotlib.pyplot as plt


def plot_histogram(histogram):
    labels = list(histogram[0][1].keys())  # Extract labels from the first tuple
    probabilities = [entry[0] for entry in histogram]  # Extract probabilities
    costs = {label: [entry[1][label] for entry in histogram] for label in labels}  # Extract costs for each label

    # Plot each label
    for label in labels:
        plt.plot(probabilities, costs[label], label=label)

    # Add labels and legend
    plt.xlabel('Probability')
    plt.ylabel('Cost/Reward')
    plt.title('Histogram')
    plt.legend()
    plt.grid(True)

    # Show plot
    plt.show()


def plot_cdf(histogram):
    labels = list(histogram[0][1].keys())  # Extract labels from the first tuple
    probabilities = [entry[0] for entry in histogram]  # Extract probabilities
    costs = {label: [entry[1][label] for entry in histogram] for label in labels}  # Extract costs for each label

    # Plot CDF for each label
    for label in labels:
        plt.figure()  # Create a new figure for each label
        # cdf = [sum(cost <= costs[label][i] for cost in costs[label]) / len(costs[label]) for i in range(len(costs[label]))]
        plt.plot(costs[label], probabilities, label=label)

        # Add labels and legend
        plt.xlabel('Cost/Reward')
        plt.ylabel('Probability')
        plt.title(f'{label} CDF')
        plt.legend()
        plt.grid(True)

    # Show plots
    plt.show()


def main(argv):
    parser = argparse.ArgumentParser(description='Example of command-line flags in Python')
    parser.add_argument('-s', '--states', type=str, help='Path prefix for the states file')
    parser.add_argument('-m', '--perf', type=str, help='Path for the performance model spec file')

    args = parser.parse_args()

    if not args.states:
        print("--states (the path prefix for the states data) is required")
        exit(1)

    perf_model = perf.PerformanceModel()
    if args.perf:
        print("perf file", args.perf)
        perf_model = markov_chain.load_performance_model_from_file(args.perf)

    # print(perf_model)

    nodespb = markov_chain.load_nodes_from_proto_files(args.states)
    # print(nodespb)
    nodes = []
    for i, node in enumerate(nodespb.json):
        # print(i, node)
        nodes.append(json.loads(node))

    links = markov_chain.load_adj_lists_from_proto_files(args.states)

    steady_state,metrics = markov_chain.steady_state(links, perf_model)
    print(steady_state)
    print(metrics)

    steady_state_nodes = []
    for i,prob in enumerate(steady_state):
        if prob > 1e-6:
            print(f'{i:4d}: {prob:.8f} {fmt.get_state_string(nodes[i])}')
            steady_state_nodes.append((i, prob, nodes[i]))

    plot_histogram(metrics.histogram)
    plot_cdf(metrics.histogram)
    # markov_chain.create_cost_matrices(links, perf_model)
    # Time to reach steady state
    # Clone the transition matrix, and for each


if __name__ == '__main__':
    main(sys.argv)
