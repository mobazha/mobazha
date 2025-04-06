package factory

import pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"

func NewImage() *pb.Image {
	return &pb.Image{
		Filename: "image.jpg",
		Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
		Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
		Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
		Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
		Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	}
}
