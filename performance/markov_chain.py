import os
import glob
import proto.graph_pb2 as graph # Import your Protocol Buffers generated code
import proto.performance_model_pb2 as perf # Import your Protocol Buffers generated code

import yaml
import json
from google.protobuf import json_format

import numpy as np
from scipy.sparse import csr_matrix


class Metrics:
    def __init__(self):
        self.mean = {}
        self.histogram = []

    def add_histogram(self, percentile, counters):
        new_counters = {}
        for counter in counters:
            new_counters[counter] = counters[counter]
        self.histogram.append((percentile, new_counters))

    def __str__(self):
        return f"Metrics(mean={self.mean}, histogram={self.histogram})"


def load_performance_model_from_file(file_path):
    # Create an empty PerformanceModel message
    perf_model = perf.PerformanceModel()

    yaml_to_proto(file_path, perf_model)

    return perf_model


def yaml_to_proto(yaml_filename, proto_msg):
    # Read the YAML file
    with open(yaml_filename, 'r') as yaml_file:
        yaml_content = yaml.safe_load(yaml_file)

    # Convert YAML content to JSON string
    json_str = json.dumps(yaml_content)

    # Parse JSON string into Proto message
    json_format.Parse(json_str, proto_msg)

    return proto_msg


def load_adj_lists_from_proto_files(path_prefix):
    pattern = os.path.join(f"{path_prefix}*adjacency_lists_*.pb")
    links = graph.Links()
    return load_proto_files(pattern, links)


def load_nodes_from_proto_files(path_prefix):
    pattern = os.path.join(f"{path_prefix}*nodes_*.pb")
    pb = graph.Nodes()
    return load_proto_files(pattern, pb)


def load_proto_files(pattern, pb):
    file_paths = glob.glob(pattern)
    for file_path in file_paths:
        with open(file_path, "rb") as f:
            pb.MergeFromString(f.read())
            # print(pb)
    return pb


# def update_transition_matrix(matrix, links):
#     for link in links.links:
#         matrix[link.src][link.dest] += link.weight


def update_transition_matrix(matrix, links, model):
    total_prob = np.zeros(links.total_nodes, dtype=np.double)
    missing_prob = np.zeros(links.total_nodes, dtype=np.double)
    missing_count = np.zeros(links.total_nodes, dtype=np.int32)
    out_degree = np.zeros(links.total_nodes, dtype=np.int32)
    link_probs = {}

    for i,link in enumerate(links.links):
        # print(i, link)
        out_degree[link.src] = round(1 / link.weight)

        if len(link.labels) == 0:
            missing_count[link.src] += 1
            continue

        link_prob = 0.0
        for label in link.labels:
            # print(label, model.configs[label])
            link_prob += model.configs[label].probability

        total_prob[link.src] += link_prob
        # matrix[link.src][link.dest] += link_prob
        link_probs[i] = link_prob

    for i in range(links.total_nodes):
        if total_prob[i] == 0:
            missing_count[i] = out_degree[i]
        if missing_count[i] > 0:
            # missingProb = (1.0 - totalProb) / float64(missingCount)
            missing_prob[i] = (1.0 - total_prob[i]) / missing_count[i]

    for i,link in enumerate(links.links):
        if i in link_probs and total_prob[link.src] > 0:
            matrix[link.src][link.dest] += link_probs[i]
        else:
            matrix[link.src][link.dest] += missing_prob[link.src]

    print('total_prob\n', total_prob)
    print('missing_prob\n', missing_prob)
    print('missing_count\n', missing_count)
    print('out_degree\n', out_degree)
    print('link_probs\n', link_probs)


def create_transition_matrix(links, model):
    matrix = np.zeros((links.total_nodes, links.total_nodes), dtype=np.double)
    update_transition_matrix(matrix, links, model)
    return matrix


def create_cost_matrices(links, model):
    if not model:
        return {}

    cost_matrices = {}
    # print('model', model)
    for label in model.configs:
        # print(label, model.configs[label])
        for counter in model.configs[label].counters:
            if counter not in cost_matrices:
                cost_matrices[counter] = np.zeros((links.total_nodes, links.total_nodes), dtype=np.double)

    # for each link, iterate over each label, and add the cost to the cost matrix
    for link in links.links:
        for label in link.labels:
            if label not in model.configs:
                continue
            config = model.configs[label]
            for counter in config.counters:
                # print(counter, link.src, link.dest, config.counters[counter])
                cost_matrices[counter][link.src][link.dest] += config.counters[counter].numeric

    print('cost_matrices', cost_matrices)
    return cost_matrices


def analyze(matrix, cost_matrices, initial_distribution, num_iterations=2000, tolerance=1e-6):
    """
    Runs the power iteration algorithm to analyze the markov chain. Specifically, it does two things:
    1. It computes the steady state distribution of the markov chain.
    2. It computes the metrics (mean/histogram) of each counter in the performance model.

    Steady state distribution is computed as next_dist = current_dist * transition_matrix.
    Expected cost of each transition = cost of each transition * probability of the transition
    expected_cost_to_stead_state = current_dist * (expected_transition_cost) for each label.

    To compute the histogram, the logic is a bit complicated. In each iteration, for each absorbing state,
    set the current_dist to 0 and normalize the rest of the states' probability to 1, and continue the iteration.

    :param matrix:
    :param cost_matrices:
    :param initial_distribution:
    :param num_iterations:
    :param tolerance:
    :return:
    """
    n = len(matrix)
    dist = initial_distribution.copy()
    alt_dist = initial_distribution.copy()

    expected_cost_matrices = {}
    mean_counters = {}
    raw_counters = {}
    metrics = Metrics()
    for counter in cost_matrices:
        expected_cost_matrices[counter] = cost_matrices[counter] * matrix
        mean_counters[counter] = 0.0
        raw_counters[counter] = 0.0

    prev_termination_prob = 0.0
    change = 1.0
    for i in range(num_iterations):
        termination_prob = 0.0
        for counter in cost_matrices:
            mean_counters[counter] += sum(np.dot(dist, expected_cost_matrices[counter]))
            raw_counters[counter] += sum(np.dot(alt_dist, expected_cost_matrices[counter]))

        new_dist = np.dot(dist, matrix)
        alt_dist = np.dot(alt_dist, matrix)

        for j in range(n):
            if matrix[j][j] == 1:
                # print(new_dist[j])
                termination_prob += new_dist[j]
                alt_dist[j] = 0.0
        total_prob = sum(alt_dist)
        for j in range(n):
            alt_dist[j] = alt_dist[j] / total_prob
            # mean_counters[counter] += sum(cost)

        # print(i, dist)
        # print(i, new_dist)
        # print(i, alt_dist)
        # print(i, mean_counters)

        if termination_prob > prev_termination_prob:
            metrics.add_histogram(termination_prob, raw_counters)

        prev_termination_prob = termination_prob

        change = np.linalg.norm(new_dist - dist)
        dist = new_dist
        if change < tolerance:
            print(f"Convergence reached after {i+1} iterations.")
            break

    if change >= tolerance:
        print(f"Convergence not reached after {num_iterations} iterations.")

    metrics.mean = mean_counters
    return dist,metrics


def initial_distribution_from_init_state(n):
    # Create a vector of length n with all elements = 0 except the first element = 1
    v = np.zeros(n)
    v[0] = 1
    return v


def steady_state(links, perf_model):
    matrix = create_transition_matrix(links, perf_model)
    cost_matrices = create_cost_matrices(links, perf_model)
    # print(matrix)
    initial_distribution = initial_distribution_from_init_state(links.total_nodes)
    # print(initial_distribution)
    prob,metrics = analyze(matrix, cost_matrices, initial_distribution)
    return prob,metrics


def steady_state_cost_metrics(links, perf_model):
    matrix = create_transition_matrix(links, perf_model)
    # print(matrix)
    initial_distribution = initial_distribution_from_init_state(links.total_nodes)
    # print(initial_distribution)
    prob = analyze(matrix, initial_distribution)
    return prob
