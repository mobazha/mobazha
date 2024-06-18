// Reference: https://github.com/mikeapr4/vue-backbone
// When create a component, pass bb as a prop to the component.
// The passed prop should be a function, and return Backbone model/collection mappings.
// It would create a _key of each model or collection inside component's data option, 
// and each mapping key of original model or collection data in component's computed option.

// import modelProxy from "./model-proxy.js";
// import collectionProxy from "./collection-proxy.js";

import Backbone from 'backbone';
import _ from 'underscore';

/**
 * Default values for the possible options passed in Vue.use
 */
var opts = {
	dataPrefix: "_",
};

/**
 * Functions to retrieve the underlying POJO
 * beneath the Backbone objects
 */

function rawSrcModel(model) {
	return model.toJSON2();
}

function rawSrcCollection(collection) {
	return collection.map(rawSrcModel);
}

function rawSrc(bb) {
	return bb.models ? rawSrcCollection(bb) : rawSrcModel(bb);
}

/**
 * When Proxies are enabled, the computed value is the most
 * practical way to access the proxy (functionality and data together).
 * However without proxies, the raw data should be accessible
 * via the `bb` options key directly, and the instance (functionality)
 * will be accessible via the vm.$bb[key] property.
 */
function getDataKey(key) {
	return opts.dataPrefix + key;
}

/**
 * Setup handlers for Backbone events, so that Vue keeps sync.
 * Also ensure Models are mapped.
 */

function bindCollectionToVue(vm, key, ctx, bb) {
	// Changes to collection array will require a full reset (for reactivity)
	ctx.onchange = () => {
		vm.$data[getDataKey(key)] = rawSrcCollection(bb);
	};
	bb.on("reset sort remove add", ctx.onchange);
}

/**
 * As VueBackbone can't support reactivity on new attributes added to a Backbone
 * Model, there's a safety with warning for it.
 */
function bindModelToVue(vm, key, ctx, bb) {
	ctx.onchange = () => {
		// Test for new attribute
		if (bb.keys().length > Object.keys(bb._previousAttributes).length) {
			// Not an error, as it may be the case this attribute is not needed for Vue at all
			console.warn(
				"VueBackbone: Adding new Model attributes after binding is not supported, provide defaults for all properties"
			);
		}

		vm.$data[getDataKey(key)] = rawSrc(bb);
	};

	bb.on("change", ctx.onchange);
}

function bindBBToVue(vm, key) {
	var ctx = vm._vuebackbone[key],
		bb = ctx.bb;

	bb.models ? bindCollectionToVue(vm, key, ctx, bb) : bindModelToVue(vm, key, ctx, bb);
}

/**
 * Cleanup if the Backbone link is changed, or if the Vue is destroyed
 */
function unbindBBFromVue(vm, key) {
	var ctx = vm._vuebackbone[key];
	delete vm._vuebackbone[key];
	
	if (ctx) {
		ctx.bb.off(null, ctx.onchange);
	}
}

/**
 * Update Vue data object, at this point it will already be a function (not a hash)
 * This will make the underlying source of the collection/model reactive.
 */
function extendData(vm, key) {
	var origDataFn = vm.$options.data,
		ctx = vm._vuebackbone[key],
		value = rawSrc(ctx.bb),
		dataKey = getDataKey(key);

	vm.$options.data = function() {
		let data = {},
			origData = origDataFn ? origDataFn.apply(this, arguments) : {};

		if (origData.hasOwnProperty(key)) {
			throw `VueBackbone: Property '${key}' mustn't exist within the Vue data already`;
		}
		// if (origData.hasOwnProperty(dataKey)) {
		// 	throw `VueBackbone: Property '${dataKey}' mustn't exist within the Vue data already`;
		// }
		// shallow copy (just in case)
		Object.keys(origData).forEach(attr => (data[attr] = origData[attr]));
		data[dataKey] = value;
		return data;
	};
}

/**
 * Update Vue computed functions, this will provide a handy accessor (key)
 * for mapped models of a collection, or the mapped model directly.
 *
 * Computed (this.key) access will trigger, this._key (reactive) access,
 * which means any computed values recompute.
 * In the case of Collections, the reason this is needed is that calculations in the
 * collection can work off the internal models arrays, which isn't the same as the rawSrc one
 * For Models, this access is important in the case the full model object is replaced,
 * it will ensure the computed value recomputes.
 */
function extendComputed(vm, key) {
	var ctx = vm._vuebackbone[key],
		dataKey = getDataKey(key),
		o = vm.$options;

	o.computed = o.computed || {};

	o.computed[key] = {
		get() {
			let access = vm.$data[dataKey]; // eslint-disable-line no-unused-vars
			return ctx.bb;
		},
		set(bb) {
			unbindBBFromVue(vm, key);
			ctx.bb = bb;
			vm.$data[dataKey] = rawSrc(bb);
			bindBBToVue(vm, key);
		}
	};
}

/**
 * Setup Vue and BB instance during Vue creation.
 * At this point the validation/normalization has
 * occurred.
 */
function initBBAndVue(vm, key, bb, prop) {
	vm._vuebackbone[key] = { bb: bb };

	if (!prop) {
		extendData(vm, key);

		extendComputed(vm, key);
	}
	bindBBToVue(vm, key);
}

/**
 * Vue Mixin with Global Handlers
 */
let vueBackboneMixin = {
	beforeCreate() {
		var vm = this,
			bbopts = vm.$props.bb;
		if (bbopts) {
			if (typeof bbopts !== "function") {
				throw `VueBackbone: 'bb' initialization option must be a function`;
			}
			bbopts = bbopts(); // remember, it's a function
			vm._vuebackbone = {};

			Object.keys(bbopts).forEach(key => {
				var bb = bbopts[key],
					prop = false;

				// Detect Property
				if (bb.prop === true) {
					if (!vm.$options.propsData || !vm.$options.propsData[key]) {
						throw `VueBackbone: Missing Backbone object in Vue prop '${key}'`;
					}
					bb = vm.$options.propsData[key];
					prop = true;
				}

				// Detect Model or Collection
				if (bb.on && (bb.attributes || bb.models)) {
					initBBAndVue(vm, key, bb, prop);
				} else {
					throw `VueBackbone: Unrecognized Backbone object in Vue instantiation (${key}), must be a Collection or Model`;
				}
			});
		}
	},
	unmounted: function() {
		let vm = this,
			ctx = vm._vuebackbone;
		if (ctx) {
			Object.keys(ctx).forEach(key => unbindBBFromVue(vm, key));
		}
	}
};

export function install(Vue, options) {
	for (let key in options) {
		if (options.hasOwnProperty(key)) {
			opts[key] = options[key];
		}
	}

	// https://github.com/jashkenas/backbone/issues/483#issuecomment-71374622
	Backbone.Model.prototype.toJSON2 = function() {
		if (this._isSerializing) {
			return this.id || this.cid;
		}
		this._isSerializing = true;
		var json = _.clone(this.attributes);
		_.each(json, function(value, name) {
			_.isFunction((value || "").toJSON2) && (json[name] = value.toJSON2());
		});
		this._isSerializing = false;
		return json;
	}

	Vue.mixin(vueBackboneMixin);
}

export default {
	install: install,
};
