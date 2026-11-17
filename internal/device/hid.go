package device

import (
	"fmt"

	"github.com/karalabe/hid"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

// HIDConnector opens the real mouse through the HID library.
type HIDConnector struct{}

// Connect implements Connector.
func (HIDConnector) Connect() (Transport, Info, error) {
	if !hid.Supported() {
		return nil, Info{}, fmt.Errorf("HID not supported on this build")
	}
	infos, err := hid.Enumerate(protocol.VendorID, 0)
	if err != nil {
		return nil, Info{}, fmt.Errorf("enumerate HID devices: %w", err)
	}
	byPath := make(map[string]hid.DeviceInfo, len(infos))
	cands := make([]Candidate, 0, len(infos))
	for _, d := range infos {
		key := fmt.Sprintf("%s|%d|%d", d.Path, d.UsagePage, d.Usage)
		byPath[key] = d
		cands = append(cands, Candidate{
			Path: key, ProductID: d.ProductID, Product: d.Product,
			UsagePage: d.UsagePage, Usage: d.Usage, Interface: d.Interface,
		})
	}
	selected := SelectCandidates(cands)
	if len(selected) == 0 {
		return nil, Info{}, ErrNotFound
	}
	var lastErr error
	for _, c := range selected {
		d := byPath[c.Path]
		dev, err := d.Open()
		if err != nil {
			lastErr = err
			continue
		}
		name := d.Product
		if name == "" {
			name = "Incott Mouse"
		}
		return hidTransport{dev}, Info{Name: name, ProductID: d.ProductID}, nil
	}
	return nil, Info{}, fmt.Errorf("%w: %w", ErrPermission, lastErr)
}

type hidTransport struct{ dev hid.Device }

func (t hidTransport) SendFeature(r []byte) error {
	_, err := t.dev.SendFeatureReport(r)
	return err
}

func (t hidTransport) GetFeature(b []byte) (int, error) { return t.dev.GetFeatureReport(b) }

func (t hidTransport) ReadInterrupt(b []byte, timeoutMS int) (int, error) {
	return t.dev.ReadTimeout(b, timeoutMS)
}

func (t hidTransport) Close() error { return t.dev.Close() }
