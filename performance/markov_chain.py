import os
import glob
import proto.graph_pb2 as graph # Import your Protocol Buffers generated code
import proto.performance_model_pb2 as perf # Import your Protocol Buffers generated code

import yaml
import json
from google.protobuf import json_format

import numpy as np
from scipy.sparse import csr_matrix


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
            print(pb)
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
        print(i, link)
        out_degree[link.src] = round(1 / link.weight)

        if len(link.labels) == 0:
            missing_count[link.src] += 1
            continue

        link_prob = 0.0
        for label in link.labels:
            print(label, model.configs[label])
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


def analyze(matrix, initial_distribution, num_iterations=2000, tolerance=1e-12):
    n = len(matrix)
    v = initial_distribution.copy()
    for i in range(num_iterations):
        new_dist = np.dot(v, matrix)
        change = np.linalg.norm(new_dist - v)
        if change < tolerance:
            print(f"Convergence reached after {i+1} iterations.")
            return new_dist
        v = new_dist
    return v


def initial_distribution_from_init_state(n):
    # Create a vector of length n with all elements = 0 except the first element = 1
    v = np.zeros(n)
    v[0] = 1
    return v


def steady_state(links, perf_model):
    matrix = create_transition_matrix(links, perf_model)
    print(matrix)
    initial_distribution = initial_distribution_from_init_state(links.total_nodes)
    print(initial_distribution)
    prob = analyze(matrix, initial_distribution)
    return prob
