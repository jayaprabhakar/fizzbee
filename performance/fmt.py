
def get_state_string(node):
    node_str = ""
    process = node['process']
    if 'globals' in process:
        node_str += f"state: {process['globals']} / "

    if 'returns' in process:
        node_str += f"returns: {process['returns']}"

    return node_str
