package gene

// Canonical-ABI (witcell-style) TYPED sibling path for ONE gene method:
// gene.catalog. This is ADDITIVE and OPT-IN. A gene cell keeps calling
// Register(impl) for the full msgpack surface, and ADDITIONALLY calls
// ProvideCatalogTyped(impl) to route the single "gene.catalog" method over the
// canonical-ABI path (pulp.ProvideRaw). Fiber's dispatchOnCall checks raw
// providers BEFORE msgpack, so this shadows ONLY gene.catalog: every other
// gene method — and every gene that never calls ProvideCatalogTyped — stays on
// the unchanged msgpack Provider path byte-for-byte.
//
// The pulp_post_return wasmexport this path requires is deliberately NOT
// emitted here. It must live in the cell's package main (a one-line wrapper
// over CatalogPostReturn), exactly like the witcell generator emits it per
// cell. Keeping the export out of this shared package is what preserves Pulp's
// post_return gate: importing gene must NOT pull a msgpack-only gene onto the
// typed path.
//
// Wire layout (mirrors the witcell canonical-ABI encoding; wasm32 LE):
//
//	record kv (tuple<string,string>)   align 4, size 16 { key@0, value@8 }
//	record sku            align 8, size 48 { id@0, name@8, description@16,
//	                        price-cents:s64@24, currency@32, metadata:list<kv>@40 }
//	record route-decl     align 4, size 16 { method@0, path@8 }
//	record admin-tab      align 4, size 28 { key@0, label@8, icon@16, order:s32@24 }
//	record registration-info align 4, size 48 { name@0, version@8, skus:list<sku>@16,
//	                        routes:list<route-decl>@24, admin-tabs:list<admin-tab>@32,
//	                        email-templates:list<string>@40 }
//	catalog: func() -> result<registration-info, string>
//	                        align 4, size 52 { disc:u8@0, payload@4 }

import (
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

// ---- raw linear-memory access (little-endian, wasm32) ----

func tWriteU8(a uint32, v uint8)   { *(*uint8)(unsafe.Pointer(uintptr(a))) = v }
func tWriteU32(a uint32, v uint32) { *(*uint32)(unsafe.Pointer(uintptr(a))) = v }
func tWriteU64(a uint32, v uint64) { *(*uint64)(unsafe.Pointer(uintptr(a))) = v }
func tReadU8(a uint32) uint8       { return *(*uint8)(unsafe.Pointer(uintptr(a))) }
func tReadU32(a uint32) uint32     { return *(*uint32)(unsafe.Pointer(uintptr(a))) }

// tLowerString pins a copy of str in the cell alloc table and returns its
// (ptr,len) header values.
func tLowerString(str string) (ptr, length uint32) {
	if len(str) == 0 {
		return 0, 0
	}
	p := pulp.Alloc(uint32(len(str)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(p))), len(str))
	copy(dst, str)
	return p, uint32(len(str))
}

// tWriteStringSlot lowers str and writes its (ptr,len) header at addr.
func tWriteStringSlot(a uint32, str string) {
	p, l := tLowerString(str)
	tWriteU32(a+0, p)
	tWriteU32(a+4, l)
}

func tFreeStringSlot(a uint32) { pulp.Free(tReadU32(a + 0)) }

// ---- list lowering (each returns the (ptr,len) header values) ----

// tLowerKVList lowers a map<string,string> as list<tuple<string,string>>.
// Go map iteration order is unspecified, matching the unordered map semantics.
func tLowerKVList(m map[string]string) (ptr, length uint32) {
	n := uint32(len(m))
	if n == 0 {
		return 0, 0
	}
	base := pulp.Alloc(n * 16)
	i := uint32(0)
	for k, v := range m {
		slot := base + i*16
		tWriteStringSlot(slot+0, k)
		tWriteStringSlot(slot+8, v)
		i++
	}
	return base, n
}

func tFreeKVList(a uint32) {
	base := tReadU32(a + 0)
	n := tReadU32(a + 4)
	for i := uint32(0); i < n; i++ {
		slot := base + i*16
		tFreeStringSlot(slot + 0)
		tFreeStringSlot(slot + 8)
	}
	pulp.Free(base)
}

func tLowerStringList(xs []string) (ptr, length uint32) {
	n := uint32(len(xs))
	if n == 0 {
		return 0, 0
	}
	base := pulp.Alloc(n * 8)
	for i := range xs {
		tWriteStringSlot(base+uint32(i)*8, xs[i])
	}
	return base, n
}

func tFreeStringList(a uint32) {
	base := tReadU32(a + 0)
	n := tReadU32(a + 4)
	for i := uint32(0); i < n; i++ {
		tFreeStringSlot(base + i*8)
	}
	pulp.Free(base)
}

func tLowerSKUList(xs []SKU) (ptr, length uint32) {
	n := uint32(len(xs))
	if n == 0 {
		return 0, 0
	}
	base := pulp.Alloc(n * 48)
	for i := range xs {
		slot := base + uint32(i)*48
		tWriteStringSlot(slot+0, xs[i].ID)
		tWriteStringSlot(slot+8, xs[i].Name)
		tWriteStringSlot(slot+16, xs[i].Description)
		tWriteU64(slot+24, uint64(xs[i].PriceCents))
		tWriteStringSlot(slot+32, xs[i].Currency)
		p, l := tLowerKVList(xs[i].Metadata)
		tWriteU32(slot+40, p)
		tWriteU32(slot+44, l)
	}
	return base, n
}

func tFreeSKUList(a uint32) {
	base := tReadU32(a + 0)
	n := tReadU32(a + 4)
	for i := uint32(0); i < n; i++ {
		slot := base + i*48
		tFreeStringSlot(slot + 0)
		tFreeStringSlot(slot + 8)
		tFreeStringSlot(slot + 16)
		tFreeStringSlot(slot + 32)
		tFreeKVList(slot + 40)
	}
	pulp.Free(base)
}

func tLowerRouteList(xs []RouteDecl) (ptr, length uint32) {
	n := uint32(len(xs))
	if n == 0 {
		return 0, 0
	}
	base := pulp.Alloc(n * 16)
	for i := range xs {
		slot := base + uint32(i)*16
		tWriteStringSlot(slot+0, xs[i].Method)
		tWriteStringSlot(slot+8, xs[i].Path)
	}
	return base, n
}

func tFreeRouteList(a uint32) {
	base := tReadU32(a + 0)
	n := tReadU32(a + 4)
	for i := uint32(0); i < n; i++ {
		slot := base + i*16
		tFreeStringSlot(slot + 0)
		tFreeStringSlot(slot + 8)
	}
	pulp.Free(base)
}

func tLowerAdminTabList(xs []AdminTab) (ptr, length uint32) {
	n := uint32(len(xs))
	if n == 0 {
		return 0, 0
	}
	base := pulp.Alloc(n * 28)
	for i := range xs {
		slot := base + uint32(i)*28
		tWriteStringSlot(slot+0, xs[i].Key)
		tWriteStringSlot(slot+8, xs[i].Label)
		tWriteStringSlot(slot+16, xs[i].Icon)
		tWriteU32(slot+24, uint32(int32(xs[i].Order)))
	}
	return base, n
}

func tFreeAdminTabList(a uint32) {
	base := tReadU32(a + 0)
	n := tReadU32(a + 4)
	for i := uint32(0); i < n; i++ {
		slot := base + i*28
		tFreeStringSlot(slot + 0)
		tFreeStringSlot(slot + 8)
		tFreeStringSlot(slot + 16)
	}
	pulp.Free(base)
}

// lowerCatalogResult encodes result<registration-info, string> (align 4,
// size 52) as a pinned pointer tree, returning the top record pointer.
func lowerCatalogResult(info RegistrationInfo, err error) uint32 {
	rec := pulp.Alloc(52) // zero-filled
	if err != nil {
		tWriteU8(rec+0, 1) // err arm
		tWriteStringSlot(rec+4, err.Error())
		return rec
	}
	tWriteU8(rec+0, 0) // ok arm; registration-info fields begin at rec+4
	tWriteStringSlot(rec+4, info.Name)
	tWriteStringSlot(rec+12, info.Version)
	{
		p, l := tLowerSKUList(info.SKUs)
		tWriteU32(rec+20, p)
		tWriteU32(rec+24, l)
	}
	{
		p, l := tLowerRouteList(info.Routes)
		tWriteU32(rec+28, p)
		tWriteU32(rec+32, l)
	}
	{
		p, l := tLowerAdminTabList(info.AdminTabs)
		tWriteU32(rec+36, p)
		tWriteU32(rec+40, l)
	}
	{
		p, l := tLowerStringList(info.EmailTemplates)
		tWriteU32(rec+44, p)
		tWriteU32(rec+48, l)
	}
	return rec
}

// CatalogPostReturn tree-frees the result<registration-info, string> record the
// host just lifted: top record + every string / list / nested-map sub-buffer
// pinned during lowerCatalogResult. The cell's package main must export it as
// pulp_post_return so Cell.CallTyped can tree-free after lifting, leaving zero
// leaked pins. recPtr is the top record pointer written by rawCatalog.
func CatalogPostReturn(recPtr uint32) {
	if recPtr == 0 {
		return
	}
	if tReadU8(recPtr+0) == 0 { // ok arm
		tFreeStringSlot(recPtr + 4)  // name
		tFreeStringSlot(recPtr + 12) // version
		tFreeSKUList(recPtr + 20)
		tFreeRouteList(recPtr + 28)
		tFreeAdminTabList(recPtr + 36)
		tFreeStringList(recPtr + 44)
	} else { // err arm
		tFreeStringSlot(recPtr + 4)
	}
	pulp.Free(recPtr)
}

// rawCatalog is the pulp.RawProvider registered for gene.catalog: it runs the
// typed handler, lowers the RegistrationInfo as a pinned pointer tree, and
// writes the (respPtr, respLen) out-params. The request carries no args.
func rawCatalog(fn func() (RegistrationInfo, error)) pulp.RawProvider {
	return func(argsPtr, argsLen, respPtrOut, respLenOut uint32) uint32 {
		_ = argsPtr
		_ = argsLen
		info, err := fn()
		rec := lowerCatalogResult(info, err)
		tWriteU32(respPtrOut, rec)
		tWriteU32(respLenOut, 52)
		return 0
	}
}

// ProvideCatalogTyped opts the gene's Catalog method into the canonical-ABI
// (witcell) sibling path. Call it IN ADDITION to Register(g): Register wires
// the full msgpack surface, then this overrides ONLY gene.catalog onto the
// typed path (dispatchOnCall checks raw providers first). The cell MUST also
// export pulp_post_return from its package main, delegating to
// CatalogPostReturn, or a canonical-ABI caller (Cell.CallTyped) cannot
// tree-free the response.
func ProvideCatalogTyped(g Gene) {
	pulp.ProvideRaw(FnCatalog, rawCatalog(func() (RegistrationInfo, error) {
		return g.Catalog(), nil
	}))
}
