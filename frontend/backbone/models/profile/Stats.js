import BaseModel from '../BaseModel';

export default class extends BaseModel {
  defaults() {
    return {
      followerCount: 0,
      followingCount: 0,
      listingCount: 0,
      physicalListingCount: 0,
      digitalListingCount: 0,
      serviceListingCount: 0,
      cryptocurrencyListingCount: 0,
      ratingCount: 0,
      postCount: 0,
      averageRating: 0,
    };
  }
}
