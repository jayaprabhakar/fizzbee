import json
import sys
import performance.markov_chain as markov_chain
import proto.performance_model_pb2 as perf
import performance.fmt as fmt
import argparse

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

    print(perf_model)

    nodespb = markov_chain.load_nodes_from_proto_files(args.states)
    # print(nodespb)
    nodes = []
    for i, node in enumerate(nodespb.json):
        print(i, node)
        nodes.append(json.loads(node))

    links = markov_chain.load_adj_lists_from_proto_files(args.states)

    steady_state = markov_chain.steady_state(links, perf_model)
    print(steady_state)

    for i,prob in enumerate(steady_state):
        if prob > 1e-6:
            print(f'{i:4d}: {prob:.8f} {fmt.get_state_string(nodes[i])}')


if __name__ == '__main__':
    main(sys.argv)
