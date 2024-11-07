import argparse
from devnet import start_l1_network, start_l2_network, setup_paths

def main():
    parser = argparse.ArgumentParser(description='Deploy L1 or L2 network')
    parser.add_argument('--deploy', choices=['L1', 'L2'], required=True, help='Deploy L1 or L2 network')
    args = parser.parse_args()

    paths = setup_paths()

    if args.deploy == 'L1':
        start_l1_network(paths)
        print("L1 network deployed successfully")
    elif args.deploy == 'L2':
        start_l2_network(paths)
        print("L2 network deployed successfully")

if __name__ == '__main__':
    main()
